// Package translator содержит реализацию транслятора в Z3
package translator

import (
	"fmt"

	"github.com/ebukreev/go-z3/z3"
	"symbolic-execution-course/internal/symbolic"
)

// Z3Translator транслирует символьные выражения в Z3 формулы
type Z3Translator struct {
	ctx    *z3.Context
	config *z3.Config
	vars   map[string]z3.Value // Кэш переменных
}

// NewZ3Translator создаёт новый экземпляр Z3 транслятора
func NewZ3Translator() *Z3Translator {
	config := &z3.Config{}
	ctx := z3.NewContext(config)

	return &Z3Translator{
		ctx:    ctx,
		config: config,
		vars:   make(map[string]z3.Value),
	}
}

// GetContext возвращает Z3 контекст
func (zt *Z3Translator) GetContext() interface{} {
	return zt.ctx
}

// Reset сбрасывает состояние транслятора
func (zt *Z3Translator) Reset() {
	zt.vars = make(map[string]z3.Value)
}

// Close освобождает ресурсы
func (zt *Z3Translator) Close() {
	// Z3 контекст закрывается автоматически
}

// TranslateExpression транслирует символьное выражение в Z3.
// Паники из visit-методов (например, несовместимость типов) преобразуются в ошибку.
func (zt *Z3Translator) TranslateExpression(expr symbolic.SymbolicExpression) (result interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			result = nil
			err = NewTranslationError(fmt.Sprintf("ошибка трансляции: %v", r), expr)
		}
	}()
	return expr.Accept(zt), nil
}

// VisitVariable транслирует символьную переменную в Z3 (с кэшированием по имени)
func (zt *Z3Translator) VisitVariable(expr *symbolic.SymbolicVariable) interface{} {
	if v, exists := zt.vars[expr.Name]; exists {
		return v
	}
	v := zt.createZ3Variable(expr.Name, expr.ExprType)
	zt.vars[expr.Name] = v
	return v
}

// VisitIntConstant транслирует целочисленную константу в Z3
func (zt *Z3Translator) VisitIntConstant(expr *symbolic.IntConstant) interface{} {
	return zt.ctx.FromInt(expr.Value, zt.ctx.IntSort())
}

// VisitBoolConstant транслирует булеву константу в Z3
func (zt *Z3Translator) VisitBoolConstant(expr *symbolic.BoolConstant) interface{} {
	return zt.ctx.FromBool(expr.Value)
}

// VisitBinaryOperation транслирует бинарную операцию в Z3
func (zt *Z3Translator) VisitBinaryOperation(expr *symbolic.BinaryOperation) interface{} {
	leftRaw := expr.Left.Accept(zt)
	rightRaw := expr.Right.Accept(zt)

	// Арифметические операции над int
	if left, ok := leftRaw.(z3.Int); ok {
		right, ok := rightRaw.(z3.Int)
		if !ok {
			panic(fmt.Sprintf("правый операнд '%s' не является Z3 Int", expr.Right.String()))
		}
		switch expr.Operator {
		case symbolic.ADD:
			return left.Add(right)
		case symbolic.SUB:
			return left.Sub(right)
		case symbolic.MUL:
			return left.Mul(right)
		case symbolic.DIV:
			return left.Div(right)
		case symbolic.MOD:
			return left.Mod(right)
		case symbolic.EQ:
			return left.Eq(right)
		case symbolic.NE:
			return left.NE(right)
		case symbolic.LT:
			return left.LT(right)
		case symbolic.LE:
			return left.LE(right)
		case symbolic.GT:
			return left.GT(right)
		case symbolic.GE:
			return left.GE(right)
		}
		panic(fmt.Sprintf("неподдерживаемый бинарный оператор: %s", expr.Operator))
	}

	// Сравнения над bool
	if left, ok := leftRaw.(z3.Bool); ok {
		right, ok := rightRaw.(z3.Bool)
		if !ok {
			panic(fmt.Sprintf("правый операнд '%s' не является Z3 Bool", expr.Right.String()))
		}
		switch expr.Operator {
		case symbolic.EQ:
			return left.Eq(right)
		case symbolic.NE:
			return left.NE(right)
		}
		panic(fmt.Sprintf("оператор '%s' не применим к bool операндам", expr.Operator))
	}

	panic(fmt.Sprintf("неподдерживаемый тип операндов для операции '%s': %T",
		expr.Operator, leftRaw))
}

// VisitLogicalOperation транслирует логическую операцию в Z3
func (zt *Z3Translator) VisitLogicalOperation(expr *symbolic.LogicalOperation) interface{} {
	operands := make([]z3.Bool, len(expr.Operands))
	for i, operand := range expr.Operands {
		v, err := zt.castToZ3Type(operand.Accept(zt), symbolic.BoolType)
		if err != nil {
			panic(fmt.Sprintf("операнд #%d: %v", i, err))
		}
		operands[i] = v.(z3.Bool)
	}

	switch expr.Operator {
	case symbolic.AND:
		result := operands[0]
		return result.And(operands[1:]...)
	case symbolic.OR:
		result := operands[0]
		return result.Or(operands[1:]...)
	case symbolic.NOT:
		return operands[0].Not()
	case symbolic.IMPLIES:
		return operands[0].Implies(operands[1])
	}
	panic(fmt.Sprintf("неподдерживаемый логический оператор: %s", expr.Operator))
}

// VisitUnaryOperation транслирует унарную операцию в Z3
func (zt *Z3Translator) VisitUnaryOperation(expr *symbolic.UnaryOperation) interface{} {
	operand := expr.Operand.Accept(zt)
	switch expr.Operator {
	case symbolic.UMINUS:
		v, ok := operand.(z3.Int)
		if !ok {
			panic(fmt.Sprintf("операнд '%s' не является Z3 Int", expr.Operand.String()))
		}
		return v.Neg()
	case symbolic.LNOT:
		v, ok := operand.(z3.Bool)
		if !ok {
			panic(fmt.Sprintf("операнд '%s' не является Z3 Bool", expr.Operand.String()))
		}
		return v.Not()
	}
	panic(fmt.Sprintf("неподдерживаемый унарный оператор: %s", expr.Operator))
}

// VisitRef транслирует символьную ссылку в Z3.
// Ссылка кодируется целочисленной Z3-константой (адресом объекта),
// что позволяет солверу рассуждать о равенстве ссылок и, соответственно, об алиасинге.
func (zt *Z3Translator) VisitRef(expr *symbolic.Ref) interface{} {
	name := fmt.Sprintf("ref#%d", expr.ID)
	if v, exists := zt.vars[name]; exists {
		return v
	}
	v := zt.ctx.IntConst(name)
	zt.vars[name] = v
	return v
}

// VisitArrayVariable транслирует именованный символьный массив в Z3 константу сорта массива
func (zt *Z3Translator) VisitArrayVariable(expr *symbolic.ArrayVariable) interface{} {
	if v, exists := zt.vars[expr.Name]; exists {
		return v
	}
	v := zt.ctx.Const(expr.Name, zt.arraySortToZ3(expr.Sort))
	arr, ok := v.(z3.Array)
	if !ok {
		panic(fmt.Sprintf("переменная '%s' не является Z3 Array", expr.Name))
	}
	zt.vars[expr.Name] = arr
	return arr
}

// VisitArrayStore транслирует функциональное обновление массива в Z3 store
func (zt *Z3Translator) VisitArrayStore(expr *symbolic.ArrayStore) interface{} {
	array, ok := expr.Array.Accept(zt).(z3.Array)
	if !ok {
		panic(fmt.Sprintf("операнд '%s' не является Z3 Array", expr.Array.String()))
	}
	index, ok := expr.Index.Accept(zt).(z3.Int)
	if !ok {
		panic(fmt.Sprintf("индекс '%s' не является Z3 Int", expr.Index.String()))
	}
	value, ok := expr.Value.Accept(zt).(z3.Value)
	if !ok {
		panic(fmt.Sprintf("значение '%s' не является Z3 значением", expr.Value.String()))
	}
	return array.Store(index, value)
}

// VisitArraySelect транслирует чтение из массива в Z3 select
func (zt *Z3Translator) VisitArraySelect(expr *symbolic.ArraySelect) interface{} {
	array, ok := expr.Array.Accept(zt).(z3.Array)
	if !ok {
		panic(fmt.Sprintf("операнд '%s' не является Z3 Array", expr.Array.String()))
	}
	index, ok := expr.Index.Accept(zt).(z3.Int)
	if !ok {
		panic(fmt.Sprintf("индекс '%s' не является Z3 Int", expr.Index.String()))
	}
	return array.Select(index)
}

// Вспомогательные методы

// arraySortToZ3 строит Z3 сорт массива по его символьному описанию (рекурсивно для вложенных массивов)
func (zt *Z3Translator) arraySortToZ3(sort symbolic.ArraySort) z3.Sort {
	domain := zt.elemSortToZ3(sort.Index)
	var range_ z3.Sort
	if sort.Elem == symbolic.ArrayType {
		if sort.Inner == nil {
			panic("сорт массива с элементами-массивами требует внутреннего сорта")
		}
		range_ = zt.arraySortToZ3(*sort.Inner)
	} else {
		range_ = zt.elemSortToZ3(sort.Elem)
	}
	return zt.ctx.ArraySort(domain, range_)
}

// elemSortToZ3 возвращает Z3 сорт для скалярного символьного типа
func (zt *Z3Translator) elemSortToZ3(exprType symbolic.ExpressionType) z3.Sort {
	switch exprType {
	case symbolic.IntType:
		return zt.ctx.IntSort()
	case symbolic.BoolType:
		return zt.ctx.BoolSort()
	}
	panic(fmt.Sprintf("неподдерживаемый сорт элементов: %s", exprType))
}

// createZ3Variable создаёт Z3 переменную соответствующего типа
func (zt *Z3Translator) createZ3Variable(name string, exprType symbolic.ExpressionType) z3.Value {
	switch exprType {
	case symbolic.IntType:
		return zt.ctx.IntConst(name)
	case symbolic.BoolType:
		return zt.ctx.BoolConst(name)
	}
	panic(fmt.Sprintf("неподдерживаемый тип переменной '%s': %s", name, exprType))
}

// castToZ3Type приводит значение к нужному Z3 типу
func (zt *Z3Translator) castToZ3Type(value interface{}, targetType symbolic.ExpressionType) (z3.Value, error) {
	switch targetType {
	case symbolic.IntType:
		if v, ok := value.(z3.Int); ok {
			return v, nil
		}
		return nil, fmt.Errorf("ожидался z3.Int, получен %T", value)
	case symbolic.BoolType:
		if v, ok := value.(z3.Bool); ok {
			return v, nil
		}
		return nil, fmt.Errorf("ожидался z3.Bool, получен %T", value)
	}
	return nil, fmt.Errorf("неподдерживаемый целевой тип: %s", targetType)
}
