package main

import (
	"fmt"

	"symbolic-execution-course/internal"
)

// demoSource содержит функции для демонстрации внутрипроцедурного анализа:
// ветвления, арифметику и цикл с конкретной границей
const demoSource = `
package main

func testBranching(x int) int {
	if x > 0 {
		return x * 2
	} else {
		return x * -1
	}
}

func testArithmetic(x, y int) int {
	return (x + y) * (x - y)
}

func testConcreteLoop() int {
	sum := 0
	for i := 1; i <= 3; i++ {
		sum += i
	}
	return sum
}

func testSymbolicLoop(n int) int {
	sum := 0
	for i := 0; i < n; i++ {
		sum += i
	}
	return sum
}
`

func main() {
	functions := []string{"testBranching", "testArithmetic", "testConcreteLoop", "testSymbolicLoop"}

	for _, functionName := range functions {
		fmt.Printf("=== Анализ функции %s ===\n", functionName)
		results := internal.Analyse(demoSource, functionName)
		fmt.Printf("Финальных состояний: %d\n", len(results))
		// Печатаем не больше maxPrinted состояний (для символьных циклов их может быть много)
		const maxPrinted = 5
		for i, interpreter := range results {
			if i >= maxPrinted {
				fmt.Printf("... и ещё %d состояний\n", len(results)-maxPrinted)
				break
			}
			fmt.Printf("--- Состояние #%d ---\n", i+1)
			fmt.Print(interpreter.String())
		}
		fmt.Println()
	}
}
