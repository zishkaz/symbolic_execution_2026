package memory

import (
	"testing"

	"github.com/ebukreev/go-z3/z3"
	"symbolic-execution-course/internal/symbolic"
	"symbolic-execution-course/internal/translator"
)

// checkSat кодирует ограничение в SMT через Z3Translator и возвращает результат проверки выполнимости
func checkSat(t *testing.T, zt *translator.Z3Translator, constraint symbolic.SymbolicExpression) bool {
	t.Helper()
	result, err := zt.TranslateExpression(constraint)
	if err != nil {
		t.Fatalf("Translation error: %v", err)
	}
	z3Cond, ok := result.(z3.Bool)
	if !ok {
		t.Fatalf("Expected z3.Bool, got %T", result)
	}
	solver := z3.NewSolver(zt.GetContext().(*z3.Context))
	solver.Assert(z3Cond)
	sat, err := solver.Check()
	if err != nil {
		t.Fatalf("Solver error: %v", err)
	}
	return sat
}

// eqToInt строит ограничение expr == value
func eqToInt(expr symbolic.SymbolicExpression, value int64) symbolic.SymbolicExpression {
	return symbolic.NewBinaryOperation(expr, symbolic.NewIntConstant(value), symbolic.EQ)
}

func TestAllocate(t *testing.T) {
	mem := NewSymbolicMemory()

	structRef := mem.Allocate(symbolic.StructType)
	if structRef.ExprType != symbolic.StructType {
		t.Errorf("Expected StructType, got %s", structRef.ExprType)
	}
	arrayRef := mem.Allocate(symbolic.ArrayType)
	if arrayRef.ExprType != symbolic.ArrayType {
		t.Errorf("Expected ArrayType, got %s", arrayRef.ExprType)
	}
	if structRef.ID == arrayRef.ID {
		t.Errorf("Expected distinct ref IDs, got %d twice", structRef.ID)
	}
}

func TestStructFieldAssignAndRead(t *testing.T) {
	mem := NewSymbolicMemory()
	zt := translator.NewZ3Translator()

	p := mem.AllocateStruct([]symbolic.ExpressionType{symbolic.IntType, symbolic.IntType})
	mem.AssignField(p, 0, symbolic.NewIntConstant(25))

	age := mem.GetFieldValue(p, 0)
	if !checkSat(t, zt, eqToInt(age, 25)) {
		t.Error("Expected p.age == 25 to be satisfiable")
	}
	if checkSat(t, zt, eqToInt(age, 26)) {
		t.Error("Expected p.age == 26 to be unsatisfiable")
	}

	// Незаписанное поле не ограничено: подходят оба значения
	id := mem.GetFieldValue(p, 1)
	if !checkSat(t, zt, eqToInt(id, 0)) || !checkSat(t, zt, eqToInt(id, 42)) {
		t.Error("Expected unwritten p.id to be unconstrained")
	}
}

func TestStructFieldOverwrite(t *testing.T) {
	mem := NewSymbolicMemory()
	zt := translator.NewZ3Translator()

	p := mem.AllocateStruct([]symbolic.ExpressionType{symbolic.IntType})
	mem.AssignField(p, 0, symbolic.NewIntConstant(5))
	mem.AssignField(p, 0, symbolic.NewIntConstant(2))

	value := mem.GetFieldValue(p, 0)
	if !checkSat(t, zt, eqToInt(value, 2)) {
		t.Error("Expected p.a == 2 (последняя запись) to be satisfiable")
	}
	if checkSat(t, zt, eqToInt(value, 5)) {
		t.Error("Expected p.a == 5 (затёртая запись) to be unsatisfiable")
	}
}

func TestArrayAssignAndRead(t *testing.T) {
	mem := NewSymbolicMemory()
	zt := translator.NewZ3Translator()

	array := mem.Allocate(symbolic.ArrayType)
	mem.AssignToArray(array, 5, symbolic.NewIntConstant(10))

	element := mem.GetFromArray(array, 5)
	if !checkSat(t, zt, eqToInt(element, 10)) {
		t.Error("Expected array[5] == 10 to be satisfiable")
	}
	if checkSat(t, zt, eqToInt(element, 11)) {
		t.Error("Expected array[5] == 11 to be unsatisfiable")
	}

	// Незаписанный элемент не ограничен
	other := mem.GetFromArray(array, 7)
	if !checkSat(t, zt, eqToInt(other, 10)) || !checkSat(t, zt, eqToInt(other, 20)) {
		t.Error("Expected unwritten array[7] to be unconstrained")
	}
}

// TestAliasingStruct воспроизводит пример Aliasing из homework4/examples/test_functions.go:
// foo2.a = 5; foo1.a = 2; читаем foo2.a
func TestAliasingStruct(t *testing.T) {
	mem := NewSymbolicMemory()
	zt := translator.NewZ3Translator()

	foo1 := mem.AllocateStruct([]symbolic.ExpressionType{symbolic.IntType})
	foo2 := mem.AllocateStruct([]symbolic.ExpressionType{symbolic.IntType})

	mem.AssignField(foo2, 0, symbolic.NewIntConstant(5))
	mem.AssignField(foo1, 0, symbolic.NewIntConstant(2))

	foo2a := mem.GetFieldValue(foo2, 0)
	foo2aIs2 := eqToInt(foo2a, 2)

	// Если ссылки равны (алиасинг), то последняя запись видна через обе: SAT
	refsEqual := symbolic.NewBinaryOperation(foo1, foo2, symbolic.EQ)
	withAliasing := symbolic.NewLogicalOperation(
		[]symbolic.SymbolicExpression{refsEqual, foo2aIs2}, symbolic.AND)
	if !checkSat(t, zt, withAliasing) {
		t.Error("Expected foo1 == foo2 && foo2.a == 2 to be SAT")
	}

	// Если ссылки различны, то foo2.a остаётся равным 5: ограничение UNSAT
	refsDiffer := symbolic.NewBinaryOperation(foo1, foo2, symbolic.NE)
	withoutAliasing := symbolic.NewLogicalOperation(
		[]symbolic.SymbolicExpression{refsDiffer, foo2aIs2}, symbolic.AND)
	if checkSat(t, zt, withoutAliasing) {
		t.Error("Expected foo1 != foo2 && foo2.a == 2 to be UNSAT")
	}
}

// TestAliasingArray проверяет алиасинг для массивов:
// arr1[3] = 1; arr2[3] = 2; читаем arr1[3]
func TestAliasingArray(t *testing.T) {
	mem := NewSymbolicMemory()
	zt := translator.NewZ3Translator()

	arr1 := mem.AllocateArray(symbolic.IntType)
	arr2 := mem.AllocateArray(symbolic.IntType)

	mem.AssignToArray(arr1, 3, symbolic.NewIntConstant(1))
	mem.AssignToArray(arr2, 3, symbolic.NewIntConstant(2))

	arr1elem := mem.GetFromArray(arr1, 3)
	elemIs2 := eqToInt(arr1elem, 2)

	// Если arr1 и arr2 - один и тот же массив, то читается последняя запись: SAT
	refsEqual := symbolic.NewBinaryOperation(arr1, arr2, symbolic.EQ)
	withAliasing := symbolic.NewLogicalOperation(
		[]symbolic.SymbolicExpression{refsEqual, elemIs2}, symbolic.AND)
	if !checkSat(t, zt, withAliasing) {
		t.Error("Expected arr1 == arr2 && arr1[3] == 2 to be SAT")
	}

	// Если массивы различны, то arr1[3] остаётся равным 1: ограничение UNSAT
	refsDiffer := symbolic.NewBinaryOperation(arr1, arr2, symbolic.NE)
	withoutAliasing := symbolic.NewLogicalOperation(
		[]symbolic.SymbolicExpression{refsDiffer, elemIs2}, symbolic.AND)
	if checkSat(t, zt, withoutAliasing) {
		t.Error("Expected arr1 != arr2 && arr1[3] == 2 to be UNSAT")
	}
}

// TestAliasingDifferentIndices проверяет, что записи по разным индексам не пересекаются:
// arr1[3] = 1; arr2[5] = 2; даже если arr1 == arr2, то arr1[3] == 1
func TestAliasingDifferentIndices(t *testing.T) {
	mem := NewSymbolicMemory()
	zt := translator.NewZ3Translator()

	arr1 := mem.AllocateArray(symbolic.IntType)
	arr2 := mem.AllocateArray(symbolic.IntType)

	mem.AssignToArray(arr1, 3, symbolic.NewIntConstant(1))
	mem.AssignToArray(arr2, 5, symbolic.NewIntConstant(2))

	elem := mem.GetFromArray(arr1, 3)
	constraint := symbolic.NewLogicalOperation(
		[]symbolic.SymbolicExpression{
			symbolic.NewBinaryOperation(arr1, arr2, symbolic.EQ),
			eqToInt(elem, 1),
		}, symbolic.AND)
	if !checkSat(t, zt, constraint) {
		t.Error("Expected arr1 == arr2 && arr1[3] == 1 to be SAT (записи по разным индексам независимы)")
	}
}

func TestBoolField(t *testing.T) {
	mem := NewSymbolicMemory()
	zt := translator.NewZ3Translator()

	p := mem.AllocateStruct([]symbolic.ExpressionType{symbolic.BoolType})
	mem.AssignField(p, 0, symbolic.NewBoolConstant(true))

	value := mem.GetFieldValue(p, 0)
	constraint := symbolic.NewBinaryOperation(value, symbolic.NewBoolConstant(true), symbolic.EQ)
	if !checkSat(t, zt, constraint) {
		t.Error("Expected p.flag == true to be satisfiable")
	}
}

func TestAssignFieldPanicsOnArrayRef(t *testing.T) {
	mem := NewSymbolicMemory()
	array := mem.Allocate(symbolic.ArrayType)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic on AssignField with array ref")
		}
	}()
	mem.AssignField(array, 0, symbolic.NewIntConstant(1))
}

func TestAssignFieldPanicsOnTypeMismatch(t *testing.T) {
	mem := NewSymbolicMemory()
	p := mem.AllocateStruct([]symbolic.ExpressionType{symbolic.IntType})

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic on assigning bool to int field")
		}
	}()
	mem.AssignField(p, 0, symbolic.NewBoolConstant(true))
}

func TestAssignFieldPanicsOnUnknownField(t *testing.T) {
	mem := NewSymbolicMemory()
	p := mem.AllocateStruct([]symbolic.ExpressionType{symbolic.IntType})

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic on out-of-range field index")
		}
	}()
	mem.AssignField(p, 5, symbolic.NewIntConstant(1))
}
