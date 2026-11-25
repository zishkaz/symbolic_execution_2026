package main

import (
	"fmt"

	"github.com/ebukreev/go-z3/z3"
	"symbolic-execution-course/internal/memory"
	"symbolic-execution-course/internal/symbolic"
	"symbolic-execution-course/internal/translator"
)

// solveConstraint кодирует символьное ограничение в SMT через Z3Translator и проверяет выполнимость
func solveConstraint(constraint symbolic.SymbolicExpression) bool {
	zt := translator.NewZ3Translator()
	result, err := zt.TranslateExpression(constraint)
	if err != nil {
		panic(err)
	}
	solver := z3.NewSolver(zt.GetContext().(*z3.Context))
	solver.Assert(result.(z3.Bool))
	sat, err := solver.Check()
	if err != nil {
		panic(err)
	}
	return sat
}

func main() {
	mem := memory.NewSymbolicMemory()

	// Демо 1: базовые операции с массивом
	array := mem.Allocate(symbolic.ArrayType)
	mem.AssignToArray(array, 5, symbolic.NewIntConstant(10))

	fromArray := mem.GetFromArray(array, 5)
	fmt.Printf("array[5] после записи 10: %s\n", fromArray)

	anotherFromArray := mem.GetFromArray(array, 10)
	fmt.Printf("array[10] (не записывалось): %s\n", anotherFromArray)

	// Демо 2: struct-поля
	person := mem.AllocateStruct([]symbolic.ExpressionType{symbolic.IntType, symbolic.IntType})
	mem.AssignField(person, 0, symbolic.NewIntConstant(25))
	mem.AssignField(person, 1, symbolic.NewIntConstant(1001))
	fmt.Printf("person.age: %s\n", mem.GetFieldValue(person, 0))
	fmt.Printf("person.id: %s\n", mem.GetFieldValue(person, 1))

	// Демо 3: алиасинг (пример Aliasing из homework4/examples/test_functions.go)
	// foo2.a = 5; foo1.a = 2; спрашиваем солвер, возможно ли foo2.a == 2
	foo1 := mem.AllocateStruct([]symbolic.ExpressionType{symbolic.IntType})
	foo2 := mem.AllocateStruct([]symbolic.ExpressionType{symbolic.IntType})
	mem.AssignField(foo2, 0, symbolic.NewIntConstant(5))
	mem.AssignField(foo1, 0, symbolic.NewIntConstant(2))
	foo2a := mem.GetFieldValue(foo2, 0)
	foo2aIs2 := symbolic.NewBinaryOperation(foo2a, symbolic.NewIntConstant(2), symbolic.EQ)

	refsEqual := symbolic.NewBinaryOperation(foo1, foo2, symbolic.EQ)
	withAliasing := symbolic.NewLogicalOperation(
		[]symbolic.SymbolicExpression{refsEqual, foo2aIs2}, symbolic.AND)
	fmt.Printf("foo1 == foo2 && foo2.a == 2: SAT = %t (ожидается true)\n", solveConstraint(withAliasing))

	refsDiffer := symbolic.NewBinaryOperation(foo1, foo2, symbolic.NE)
	withoutAliasing := symbolic.NewLogicalOperation(
		[]symbolic.SymbolicExpression{refsDiffer, foo2aIs2}, symbolic.AND)
	fmt.Printf("foo1 != foo2 && foo2.a == 2: SAT = %t (ожидается false)\n", solveConstraint(withoutAliasing))
}
