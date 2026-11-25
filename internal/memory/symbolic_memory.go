// Package memory содержит модель символьной памяти
package memory

import (
	"fmt"

	"symbolic-execution-course/internal/symbolic"
)

// Memory интерфейс символьной памяти
type Memory interface {
	Allocate(tpe symbolic.ExpressionType) *symbolic.Ref

	AssignField(ref *symbolic.Ref, fieldIdx int, value symbolic.SymbolicExpression)

	GetFieldValue(ref *symbolic.Ref, fieldIdx int) symbolic.SymbolicExpression

	AssignToArray(ref *symbolic.Ref, index int, value symbolic.SymbolicExpression)

	GetFromArray(ref *symbolic.Ref, index int) symbolic.SymbolicExpression
}

// fieldHeap - хип значений одного поля: символьный массив Array(Ref -> fieldType)
type fieldHeap struct {
	expr     symbolic.SymbolicExpression
	elemType symbolic.ExpressionType
}

// SymbolicMemory - модель символьной памяти из лекции.
//
// Объекты (struct) моделируются набором хипов полей: для каждого индекса поля
// заводится символьный массив Array(адрес -> значение поля). Массивы языка
// моделируются вложенным хипом Array(адрес -> Array(индекс -> элемент)).
// Адреса объектов (ссылки) кодируются целыми числами, поэтому алиасинг
// поддерживается естественно: если две ссылки равны, то чтение через любую
// из них выдаёт одно и то же значение (это выводит SMT-солвер).
type SymbolicMemory struct {
	id         int
	nextRefID  int
	fieldHeaps map[int]*fieldHeap
	fieldTypes map[int][]symbolic.ExpressionType // refID -> типы полей struct
	arrayHeaps map[symbolic.ExpressionType]symbolic.SymbolicExpression
	arrayElems map[int]symbolic.ExpressionType // refID -> тип элементов массива
}

// memoryCounter - счётчик экземпляров памяти для уникальных имён хипов
var memoryCounter int

// Проверка соответствия интерфейсу
var _ Memory = (*SymbolicMemory)(nil)

// NewSymbolicMemory создаёт новую пустую символьную память
func NewSymbolicMemory() *SymbolicMemory {
	mem := &SymbolicMemory{
		id:         memoryCounter,
		nextRefID:  0,
		fieldHeaps: make(map[int]*fieldHeap),
		fieldTypes: make(map[int][]symbolic.ExpressionType),
		arrayHeaps: make(map[symbolic.ExpressionType]symbolic.SymbolicExpression),
		arrayElems: make(map[int]symbolic.ExpressionType),
	}
	memoryCounter++
	return mem
}

// Allocate выделяет объект в символьной памяти и возвращает ссылку на него.
// Для struct-объектов без явных типов полей и массивов с не-int элементами
// используйте AllocateStruct/AllocateArray.
func (mem *SymbolicMemory) Allocate(tpe symbolic.ExpressionType) *symbolic.Ref {
	switch tpe {
	case symbolic.StructType:
		return mem.AllocateStruct(nil)
	case symbolic.ArrayType:
		// По умолчанию считаем массив целочисленным
		return mem.AllocateArray(symbolic.IntType)
	}
	panic(fmt.Sprintf("memory: cannot allocate object of type %s", tpe))
}

// AllocateStruct выделяет struct-объект с известными типами полей
func (mem *SymbolicMemory) AllocateStruct(fieldTypes []symbolic.ExpressionType) *symbolic.Ref {
	ref := symbolic.NewRef(mem.nextRefID, symbolic.StructType)
	mem.nextRefID++
	mem.fieldTypes[ref.ID] = fieldTypes
	for idx, fieldType := range fieldTypes {
		mem.ensureFieldHeap(idx, fieldType)
	}
	return ref
}

// AllocateArray выделяет массив с известным типом элементов
func (mem *SymbolicMemory) AllocateArray(elemType symbolic.ExpressionType) *symbolic.Ref {
	ref := symbolic.NewRef(mem.nextRefID, symbolic.ArrayType)
	mem.nextRefID++
	mem.arrayElems[ref.ID] = elemType
	mem.ensureArrayHeap(elemType)
	return ref
}

// AssignField записывает значение в поле struct-объекта: heap[fieldIdx] = store(heap[fieldIdx], ref, value)
func (mem *SymbolicMemory) AssignField(ref *symbolic.Ref, fieldIdx int, value symbolic.SymbolicExpression) {
	mem.checkRef(ref, symbolic.StructType)
	if value == nil {
		panic(fmt.Sprintf("memory: nil value assigned to field %d of %s", fieldIdx, ref.String()))
	}
	if types := mem.fieldTypes[ref.ID]; len(types) > 0 && fieldIdx >= len(types) {
		panic(fmt.Sprintf("memory: field index %d out of range for %s (%d fields)", fieldIdx, ref.String(), len(types)))
	}
	heap := mem.ensureFieldHeap(fieldIdx, value.Type())
	if heap.elemType != value.Type() {
		panic(fmt.Sprintf("memory: field %d has type %s, cannot assign %s", fieldIdx, heap.elemType, value.Type()))
	}
	heap.expr = symbolic.NewArrayStore(heap.expr, ref, value)
}

// GetFieldValue читает значение поля struct-объекта: select(heap[fieldIdx], ref)
func (mem *SymbolicMemory) GetFieldValue(ref *symbolic.Ref, fieldIdx int) symbolic.SymbolicExpression {
	mem.checkRef(ref, symbolic.StructType)
	heap := mem.ensureFieldHeap(fieldIdx, symbolic.IntType)
	return symbolic.NewArraySelect(heap.expr, ref, heap.elemType)
}

// AssignToArray записывает значение в элемент массива:
// arrayHeap = store(arrayHeap, ref, store(select(arrayHeap, ref), index, value))
func (mem *SymbolicMemory) AssignToArray(ref *symbolic.Ref, index int, value symbolic.SymbolicExpression) {
	mem.checkRef(ref, symbolic.ArrayType)
	if value == nil {
		panic(fmt.Sprintf("memory: nil value assigned to array %s", ref.String()))
	}
	if elemType, ok := mem.arrayElems[ref.ID]; ok && elemType != value.Type() {
		panic(fmt.Sprintf("memory: array %s has elements of type %s, cannot assign %s", ref.String(), elemType, value.Type()))
	}
	heap := mem.ensureArrayHeap(value.Type())
	inner := symbolic.NewArraySelect(heap, ref, symbolic.ArrayType)
	updated := symbolic.NewArrayStore(inner, symbolic.NewIntConstant(int64(index)), value)
	mem.arrayHeaps[value.Type()] = symbolic.NewArrayStore(heap, ref, updated)
}

// GetFromArray читает элемент массива: select(select(arrayHeap, ref), index)
func (mem *SymbolicMemory) GetFromArray(ref *symbolic.Ref, index int) symbolic.SymbolicExpression {
	mem.checkRef(ref, symbolic.ArrayType)
	elemType, ok := mem.arrayElems[ref.ID]
	if !ok {
		// Тип элементов неизвестен (ref получен не через AllocateArray) - считаем int
		elemType = symbolic.IntType
	}
	heap := mem.ensureArrayHeap(elemType)
	inner := symbolic.NewArraySelect(heap, ref, symbolic.ArrayType)
	return symbolic.NewArraySelect(inner, symbolic.NewIntConstant(int64(index)), elemType)
}

// Вспомогательные методы

// checkRef проверяет, что ссылка указывает на объект ожидаемого типа
func (mem *SymbolicMemory) checkRef(ref *symbolic.Ref, expected symbolic.ExpressionType) {
	if ref == nil {
		panic("memory: nil ref")
	}
	if ref.ExprType != expected {
		panic(fmt.Sprintf("memory: expected ref to %s, got %s", expected, ref.String()))
	}
}

// ensureFieldHeap возвращает хип поля, создавая его при первом обращении
func (mem *SymbolicMemory) ensureFieldHeap(fieldIdx int, elemType symbolic.ExpressionType) *fieldHeap {
	if heap, ok := mem.fieldHeaps[fieldIdx]; ok {
		return heap
	}
	name := fmt.Sprintf("mem%d.field.%d", mem.id, fieldIdx)
	heap := &fieldHeap{
		expr:     symbolic.NewArrayVariable(name, symbolic.ArraySort{Index: symbolic.IntType, Elem: elemType}),
		elemType: elemType,
	}
	mem.fieldHeaps[fieldIdx] = heap
	return heap
}

// ensureArrayHeap возвращает хип элементов массивов, создавая его при первом обращении
func (mem *SymbolicMemory) ensureArrayHeap(elemType symbolic.ExpressionType) symbolic.SymbolicExpression {
	if heap, ok := mem.arrayHeaps[elemType]; ok {
		return heap
	}
	name := fmt.Sprintf("mem%d.array.%s", mem.id, elemType)
	heap := symbolic.NewArrayVariable(name, symbolic.ArraySort{
		Index: symbolic.IntType,
		Elem:  symbolic.ArrayType,
		Inner: &symbolic.ArraySort{Index: symbolic.IntType, Elem: elemType},
	})
	mem.arrayHeaps[elemType] = heap
	return heap
}
