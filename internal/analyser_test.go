package internal

import (
	"testing"
)

// collectByPC группирует финальные состояния по строковому представлению path condition
func collectByPC(results []Interpreter) map[string][]Interpreter {
	grouped := make(map[string][]Interpreter)
	for _, result := range results {
		pc := result.PathCondition.String()
		grouped[pc] = append(grouped[pc], result)
	}
	return grouped
}

func TestAnalyseBranching(t *testing.T) {
	source := `
package main

func testFunction(x int) int {
	if x > 0 {
		return x * 2
	} else {
		return x * -1
	}
}
`
	results := Analyse(source, "testFunction")

	if len(results) != 2 {
		t.Fatalf("Ожидалось 2 финальных состояния, получено %d", len(results))
	}

	grouped := collectByPC(results)

	trueBranch, ok := grouped["(x > 0)"]
	if !ok || len(trueBranch) != 1 {
		t.Fatalf("Ожидалась одна true-ветка с path condition '(x > 0)', получено: %v", grouped)
	}
	if ret := trueBranch[0].CallStack[0].ReturnValue.String(); ret != "(x * 2)" {
		t.Errorf("Ожидалось возвращаемое значение '(x * 2)', получено '%s'", ret)
	}

	falseBranch, ok := grouped["!(x > 0)"]
	if !ok || len(falseBranch) != 1 {
		t.Fatalf("Ожидалась одна false-ветка с path condition '!(x > 0)', получено: %v", grouped)
	}
	if ret := falseBranch[0].CallStack[0].ReturnValue.String(); ret != "(x * -1)" {
		t.Errorf("Ожидалось возвращаемое значение '(x * -1)', получено '%s'", ret)
	}
}

func TestAnalyseArithmetic(t *testing.T) {
	source := `
package main

func testFunction(x, y int) int {
	return (x + y) * (x - y)
}
`
	results := Analyse(source, "testFunction")

	if len(results) != 1 {
		t.Fatalf("Ожидалось 1 финальное состояние, получено %d", len(results))
	}
	if pc := results[0].PathCondition.String(); pc != "true" {
		t.Errorf("Ожидалось path condition 'true', получено '%s'", pc)
	}
	if ret := results[0].CallStack[0].ReturnValue.String(); ret != "((x + y) * (x - y))" {
		t.Errorf("Ожидалось возвращаемое значение '((x + y) * (x - y))', получено '%s'", ret)
	}
}

func TestAnalyseConcreteLoop(t *testing.T) {
	source := `
package main

func testFunction() int {
	sum := 0
	for i := 1; i <= 3; i++ {
		sum += i
	}
	return sum
}
`
	results := Analyse(source, "testFunction")

	if len(results) != 1 {
		t.Fatalf("Ожидалось 1 финальное состояние, получено %d", len(results))
	}
	// Все значения конкретные, поэтому благодаря свёртке констант результат вычисляется полностью
	if ret := results[0].CallStack[0].ReturnValue.String(); ret != "6" {
		t.Errorf("Ожидалось возвращаемое значение '6', получено '%s'", ret)
	}
}

func TestAnalyseSymbolicLoopTerminates(t *testing.T) {
	source := `
package main

func testFunction(n int) int {
	sum := 0
	for i := 0; i < n; i++ {
		sum += i
	}
	return sum
}
`
	// Цикл с символьной границей: анализ обязан завершиться
	// за счёт глобального лимита шагов
	results := Analyse(source, "testFunction")

	if len(results) == 0 {
		t.Fatal("Ожидалось хотя бы одно финальное состояние")
	}

	// Состояние с выходом из цикла на нулевой итерации (n <= 0) должно присутствовать
	grouped := collectByPC(results)
	if _, ok := grouped["!(0 < n)"]; !ok {
		t.Errorf("Ожидалось состояние с path condition '!(0 < n)', получено: %v", grouped)
	}
}
