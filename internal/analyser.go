package internal

import (
	"container/heap"
	"fmt"
	"go/types"

	"golang.org/x/tools/go/ssa"
	"symbolic-execution-course/internal/symbolic"
	ssabuilder "symbolic-execution-course/internal/ssa"
	"symbolic-execution-course/internal/translator"
)

// maxAnalysisSteps — глобальный лимит на количество шагов интерпретации.
// Стратегия остановки: анализ завершается, когда очередь состояний пуста,
// либо когда выполнено maxAnalysisSteps шагов интерпретации. Это гарантирует
// терминацию даже для циклов с символьной границей: состояния, оставшиеся
// в очереди на момент исчерпания лимита, просто отбрасываются.
// Значение подобрано так, чтобы анализ оставался быстрым: проверка выполнимости
// веток через Z3 дорожает с ростом path condition, поэтому большой лимит
// приводит к квадратичным затратам на глубоких развёртках циклов.
const maxAnalysisSteps = 1000

type Analyser struct {
	Package      *ssa.Package
	StatesQueue  PriorityQueue
	PathSelector PathSelector
	Results      []Interpreter
	Z3Translator *translator.Z3Translator
}

func Analyse(source string, functionName string) []Interpreter {
	builder := ssabuilder.NewBuilder()
	fn, err := builder.ParseAndBuildSSA(source, functionName)
	if err != nil {
		panic(fmt.Sprintf("не удалось построить SSA для функции %s: %v", functionName, err))
	}

	analyser := &Analyser{
		StatesQueue:  make(PriorityQueue, 0),
		PathSelector: &DfsPathSelector{},
		Results:      make([]Interpreter, 0),
		Z3Translator: translator.NewZ3Translator(),
	}
	heap.Init(&analyser.StatesQueue)

	// Начальное состояние: параметры функции — свежие символьные переменные,
	// path condition = true, исполнение начинается с первого блока функции.
	locals := make(map[string]symbolic.SymbolicExpression)
	for _, param := range fn.Params {
		locals[param.Name()] = symbolic.NewSymbolicVariable(param.Name(), ssaTypeToSymbolic(param.Type()))
	}

	initial := Interpreter{
		CallStack: []CallStackFrame{{
			Function:     fn,
			LocalMemory:  locals,
			CurrentBlock: fn.Blocks[0],
			InstrIndex:   0,
		}},
		Analyser:      analyser,
		PathCondition: symbolic.NewBoolConstant(true),
	}
	analyser.pushState(initial)

	// Основной цикл анализатора: достаём следующее состояние из приоритетной
	// очереди, делаем шаг динамической интерпретации и складываем полученные
	// состояния обратно в очередь, пока очередь не опустеет или не сработает
	// глобальная стратегия остановки (лимит шагов).
	steps := 0
	for analyser.StatesQueue.Len() > 0 && steps < maxAnalysisSteps {
		item := heap.Pop(&analyser.StatesQueue).(*Item)
		state := item.value
		steps++

		frame := &state.CallStack[len(state.CallStack)-1]
		if frame.CurrentBlock == nil || frame.InstrIndex >= len(frame.CurrentBlock.Instrs) {
			continue
		}
		instr := frame.CurrentBlock.Instrs[frame.InstrIndex]

		for _, newState := range state.interpretDynamically(instr) {
			analyser.pushState(newState)
		}
	}

	return analyser.Results
}

// pushState добавляет состояние в очередь с приоритетом, вычисленным PathSelector'ом
func (analyser *Analyser) pushState(state Interpreter) {
	heap.Push(&analyser.StatesQueue, &Item{
		value:    state,
		priority: analyser.PathSelector.CalculatePriority(state),
	})
}

// ssaTypeToSymbolic отображает базовый Go-тип на тип символьного выражения
func ssaTypeToSymbolic(tpe types.Type) symbolic.ExpressionType {
	basic, ok := tpe.Underlying().(*types.Basic)
	if !ok {
		panic(fmt.Sprintf("неподдерживаемый тип параметра: %s (поддерживаются только int и bool)", tpe))
	}
	switch {
	case basic.Info()&types.IsBoolean != 0:
		return symbolic.BoolType
	case basic.Info()&types.IsInteger != 0:
		return symbolic.IntType
	default:
		panic(fmt.Sprintf("неподдерживаемый тип параметра: %s (поддерживаются только int и bool)", tpe))
	}
}
