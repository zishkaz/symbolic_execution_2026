// Демонстрационная программа для тестирования символьных выражений
// и их трансляции в SMT-формулы для Z3 solver (ДЗ 2)
package main

import (
	"fmt"
	"log"

	"github.com/ebukreev/go-z3/z3"
	"symbolic-execution-course/internal/symbolic"
	"symbolic-execution-course/internal/translator"
)

func main() {
	fmt.Println("=== Symbolic Expressions Demo ===")

	// Создаём простые символьные выражения
	x := symbolic.NewSymbolicVariable("x", symbolic.IntType)
	y := symbolic.NewSymbolicVariable("y", symbolic.IntType)
	five := symbolic.NewIntConstant(5)

	// Создаём выражение: x + y > 5
	sum := symbolic.NewBinaryOperation(x, y, symbolic.ADD)
	condition := symbolic.NewBinaryOperation(sum, five, symbolic.GT)

	fmt.Printf("Выражение: %s\n", condition.String())
	fmt.Printf("Тип выражения: %s\n", condition.Type())

	// Создаём Z3 транслятор
	zt := translator.NewZ3Translator()
	defer zt.Close()

	// Транслируем в Z3
	z3Expr, err := zt.TranslateExpression(condition)
	if err != nil {
		log.Fatalf("Ошибка трансляции: %v", err)
	}
	fmt.Printf("Z3 выражение создано: %T\n", z3Expr)

	// Создаём более сложное выражение: (x > 0) && (y < 10)
	zero := symbolic.NewIntConstant(0)
	ten := symbolic.NewIntConstant(10)

	cond1 := symbolic.NewBinaryOperation(x, zero, symbolic.GT)
	cond2 := symbolic.NewBinaryOperation(y, ten, symbolic.LT)

	andExpr := symbolic.NewLogicalOperation([]symbolic.SymbolicExpression{cond1, cond2}, symbolic.AND)

	fmt.Printf("Сложное выражение: %s\n", andExpr.String())

	// Транслируем сложное выражение
	z3AndExpr, err := zt.TranslateExpression(andExpr)
	if err != nil {
		log.Fatalf("Ошибка трансляции сложного выражения: %v", err)
	}
	fmt.Printf("Сложное Z3 выражение создано: %T\n", z3AndExpr)

	// === Интеграция с solver: проверяем выполнимость и получаем модель ===
	fmt.Println("\n=== Проверка выполнимости через Z3 Solver ===")

	ctx := zt.GetContext().(*z3.Context)
	solver := z3.NewSolver(ctx)

	// Задача 1: найти x, y such that x + y > 5
	fmt.Println("\nЗадача 1: существуют ли x, y такие что x + y > 5?")
	solver.Assert(z3Expr.(z3.Bool))
	printModel(solver, zt, []string{"x", "y"})

	// Задача 2: (x > 0) && (y < 10)
	fmt.Println("\nЗадача 2: существуют ли x, y такие что (x > 0) && (y < 10)?")
	solver.Reset()
	solver.Assert(z3AndExpr.(z3.Bool))
	printModel(solver, zt, []string{"x", "y"})

	// Задача 3: противоречивое условие x > 0 && !(x > 0)
	fmt.Println("\nЗадача 3: условие x > 0 && !(x > 0) (ожидается UNSAT)")
	notCond1 := symbolic.NewLogicalOperation([]symbolic.SymbolicExpression{cond1}, symbolic.NOT)
	contradiction := symbolic.NewLogicalOperation(
		[]symbolic.SymbolicExpression{cond1, notCond1}, symbolic.AND)
	z3Contradiction, err := zt.TranslateExpression(contradiction)
	if err != nil {
		log.Fatalf("Ошибка трансляции: %v", err)
	}
	solver.Reset()
	solver.Assert(z3Contradiction.(z3.Bool))
	sat, err := solver.Check()
	if err != nil {
		log.Fatalf("Ошибка solver: %v", err)
	}
	if sat {
		fmt.Println("SAT - неожиданно!")
	} else {
		fmt.Println("UNSAT - условие противоречиво, как и ожидалось")
	}

	// Задача 4: унарные операции и импликация: !(x < 0) => x >= 0
	fmt.Println("\nЗадача 4: !(x < 0) => x >= 0 (ожидается SAT, тавтология)")
	xNeg := symbolic.NewBinaryOperation(x, zero, symbolic.LT)
	antecedent := symbolic.NewLogicalOperation([]symbolic.SymbolicExpression{xNeg}, symbolic.NOT)
	consequent := symbolic.NewBinaryOperation(x, zero, symbolic.GE)
	implies := symbolic.NewLogicalOperation(
		[]symbolic.SymbolicExpression{antecedent, consequent}, symbolic.IMPLIES)
	fmt.Printf("Выражение: %s\n", implies.String())

	z3Implies, err := zt.TranslateExpression(implies)
	if err != nil {
		log.Fatalf("Ошибка трансляции: %v", err)
	}
	solver.Reset()
	solver.Assert(z3Implies.(z3.Bool))
	sat, err = solver.Check()
	if err != nil {
		log.Fatalf("Ошибка solver: %v", err)
	}
	if sat {
		fmt.Println("SAT - импликация выполнима")
	} else {
		fmt.Println("UNSAT - неожиданно!")
	}

	// Задача 5: арифметика с делением по модулю (x % 2 == 0) && (y % 3 == 1)
	fmt.Println("\nЗадача 5: (x % 2 == 0) && (y % 3 == 1) - модульная арифметика")
	two := symbolic.NewIntConstant(2)
	three := symbolic.NewIntConstant(3)
	one := symbolic.NewIntConstant(1)
	xMod2 := symbolic.NewBinaryOperation(x, two, symbolic.MOD)
	yMod3 := symbolic.NewBinaryOperation(y, three, symbolic.MOD)
	eq1 := symbolic.NewBinaryOperation(xMod2, zero, symbolic.EQ)
	eq2 := symbolic.NewBinaryOperation(yMod3, one, symbolic.EQ)
	modExpr := symbolic.NewLogicalOperation([]symbolic.SymbolicExpression{eq1, eq2}, symbolic.AND)
	fmt.Printf("Выражение: %s\n", modExpr.String())

	z3ModExpr, err := zt.TranslateExpression(modExpr)
	if err != nil {
		log.Fatalf("Ошибка трансляции: %v", err)
	}
	solver.Reset()
	solver.Assert(z3ModExpr.(z3.Bool))
	printModel(solver, zt, []string{"x", "y"})

	fmt.Println("\n=== Демо завершено ===")
}

// printModel проверяет выполнимость ограничений и выводит значения переменных из модели
func printModel(solver *z3.Solver, zt *translator.Z3Translator, varNames []string) {
	sat, err := solver.Check()
	if err != nil {
		log.Fatalf("Ошибка solver: %v", err)
	}
	if !sat {
		fmt.Println("UNSAT - решений нет")
		return
	}

	model := solver.Model()
	fmt.Println("SAT - решение найдено:")
	for _, name := range varNames {
		v, err := zt.TranslateExpression(symbolic.NewSymbolicVariable(name, symbolic.IntType))
		if err != nil {
			log.Fatalf("Ошибка получения переменной %s: %v", name, err)
		}
		value := model.Eval(v.(z3.Int), true)
		fmt.Printf("  %s = %s\n", name, value.String())
	}
}
