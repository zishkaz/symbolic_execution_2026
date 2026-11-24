package internal

import (
	"fmt"
	"go/constant"
	"go/token"
	"sort"
	"strings"

	"github.com/ebukreev/go-z3/z3"
	"golang.org/x/tools/go/ssa"
	"symbolic-execution-course/internal/memory"
	"symbolic-execution-course/internal/symbolic"
)

type Interpreter struct {
	CallStack     []CallStackFrame
	Analyser      *Analyser
	PathCondition symbolic.SymbolicExpression
	Heap          memory.Memory
}

type CallStackFrame struct {
	Function    *ssa.Function
	LocalMemory map[string]symbolic.SymbolicExpression
	ReturnValue symbolic.SymbolicExpression

	// Позиция исполнения: текущий базовый блок и индекс инструкции в нём.
	// PrevBlock — блок, из которого мы пришли в CurrentBlock (нужен для ssa.Phi).
	CurrentBlock *ssa.BasicBlock
	PrevBlock    *ssa.BasicBlock
	InstrIndex   int
}

// interpretDynamically выполняет один шаг интерпретации ssa-инструкции
// и возвращает набор новых состояний интерпретатора
func (interpreter *Interpreter) interpretDynamically(element ssa.Instruction) []Interpreter {
	switch instr := element.(type) {
	case *ssa.BinOp:
		// Бинарная операция: вычисляем символьно и кладём в локальную память
		left := interpreter.resolveExpression(instr.X)
		right := interpreter.resolveExpression(instr.Y)
		interpreter.currentFrame().LocalMemory[instr.Name()] = computeBinaryOp(instr.Op, instr.Name(), left, right)
		return interpreter.advance()

	case *ssa.UnOp:
		// Унарная операция: -x, !x, ^x, +x
		operand := interpreter.resolveExpression(instr.X)
		interpreter.currentFrame().LocalMemory[instr.Name()] = computeUnaryOp(instr.Op, operand)
		return interpreter.advance()

	case *ssa.Convert:
		// Преобразование типа: грубое приближение — значение не меняется
		interpreter.currentFrame().LocalMemory[instr.Name()] = interpreter.resolveExpression(instr.X)
		return interpreter.advance()

	case *ssa.ChangeType:
		interpreter.currentFrame().LocalMemory[instr.Name()] = interpreter.resolveExpression(instr.X)
		return interpreter.advance()

	case *ssa.Phi:
		// Phi-узел: выбираем значение по блоку-предшественнику
		frame := interpreter.currentFrame()
		edgeIdx := -1
		for i, pred := range frame.CurrentBlock.Preds {
			if pred == frame.PrevBlock {
				edgeIdx = i
				break
			}
		}
		if edgeIdx == -1 {
			panic(fmt.Sprintf("ssa.Phi %s: предыдущий блок не найден среди предшественников", instr.Name()))
		}
		frame.LocalMemory[instr.Name()] = interpreter.resolveExpression(instr.Edges[edgeIdx])
		return interpreter.advance()

	case *ssa.Jump:
		// Безусловный переход на единственного наследника блока
		interpreter.goToBlock(instr.Block().Succs[0])
		return []Interpreter{*interpreter}

	case *ssa.If:
		return interpreter.interpretIf(instr)

	case *ssa.Return:
		// Состояние завершено: сохраняем возвращаемое значение в результаты
		frame := interpreter.currentFrame()
		switch len(instr.Results) {
		case 0:
			frame.ReturnValue = nil
		case 1:
			frame.ReturnValue = interpreter.resolveExpression(instr.Results[0])
		default:
			panic(fmt.Sprintf("функции с несколькими возвращаемыми значениями не поддерживаются: %s", instr))
		}
		interpreter.Analyser.Results = append(interpreter.Analyser.Results, *interpreter)
		return nil

	case *ssa.Panic:
		// Путь аварийно завершён — отбрасываем состояние
		return nil

	case *ssa.Call:
		panic(fmt.Sprintf("вызовы функций не поддерживаются во внутрипроцедурном анализе: %s", instr))

	default:
		panic(fmt.Sprintf("неподдерживаемая ssa-инструкция %T: %s", element, element))
	}
}

// resolveExpression по ssa-значению строит символьное выражение
func (interpreter *Interpreter) resolveExpression(value ssa.Value) symbolic.SymbolicExpression {
	switch v := value.(type) {
	case *ssa.Const:
		return constToSymbolic(v)
	default:
		// Все остальные значения (параметры, результаты BinOp/UnOp/Phi и т.д.)
		// уже должны быть вычислены и лежать в локальной памяти под своим именем
		if expr, ok := interpreter.currentFrame().LocalMemory[value.Name()]; ok {
			return expr
		}
		panic(fmt.Sprintf("не удалось разрешить значение %s (%T): отсутствует в локальной памяти", value.Name(), value))
	}
}

// interpretIf обрабатывает условный переход: разветвляет состояние на две ветки
// с проверкой выполнимости каждой ветки через Z3
func (interpreter *Interpreter) interpretIf(instr *ssa.If) []Interpreter {
	cond := interpreter.resolveExpression(instr.Cond)
	trueBlock := instr.Block().Succs[0]
	falseBlock := instr.Block().Succs[1]

	// Конкретное условие — переходим только по одной ветке
	if boolConst, ok := cond.(*symbolic.BoolConstant); ok {
		if boolConst.Value {
			interpreter.goToBlock(trueBlock)
		} else {
			interpreter.goToBlock(falseBlock)
		}
		return []Interpreter{*interpreter}
	}

	var result []Interpreter

	// true-ветка: path condition усиливается самим условием
	truePC := extendPathCondition(interpreter.PathCondition, cond)
	if interpreter.isFeasible(truePC) {
		clone := interpreter.clone()
		clone.PathCondition = truePC
		clone.goToBlock(trueBlock)
		result = append(result, *clone)
	}

	// false-ветка: path condition усиливается отрицанием условия
	falsePC := extendPathCondition(interpreter.PathCondition, negate(cond))
	if interpreter.isFeasible(falsePC) {
		clone := interpreter.clone()
		clone.PathCondition = falsePC
		clone.goToBlock(falseBlock)
		result = append(result, *clone)
	}

	return result
}

// isFeasible проверяет выполнимость символьного условия через Z3 solver
func (interpreter *Interpreter) isFeasible(condition symbolic.SymbolicExpression) bool {
	z3Translator := interpreter.Analyser.Z3Translator
	ctx := z3Translator.GetContext().(*z3.Context)
	solver := z3.NewSolver(ctx)

	translated, err := z3Translator.TranslateExpression(condition)
	if err != nil {
		// При ошибке трансляции считаем ветку выполнимой (не отсекаем путь)
		return true
	}
	boolExpr, ok := translated.(z3.Bool)
	if !ok {
		return true
	}
	solver.Assert(boolExpr)
	sat, err := solver.Check()
	if err != nil {
		return true
	}
	return sat
}

// computeBinaryOp отображает токен бинарной операции на символьный оператор
func computeBinaryOp(op token.Token, name string, left, right symbolic.SymbolicExpression) symbolic.SymbolicExpression {
	// Свёртка констант: если оба операнда конкретные, считаем результат сразу
	if folded, ok := foldConstants(op, left, right); ok {
		return folded
	}
	switch op {
	case token.ADD:
		return symbolic.NewBinaryOperation(left, right, symbolic.ADD)
	case token.SUB:
		return symbolic.NewBinaryOperation(left, right, symbolic.SUB)
	case token.MUL:
		return symbolic.NewBinaryOperation(left, right, symbolic.MUL)
	case token.QUO:
		return symbolic.NewBinaryOperation(left, right, symbolic.DIV)
	case token.REM:
		return symbolic.NewBinaryOperation(left, right, symbolic.MOD)
	case token.EQL:
		return symbolic.NewBinaryOperation(left, right, symbolic.EQ)
	case token.NEQ:
		return symbolic.NewBinaryOperation(left, right, symbolic.NE)
	case token.LSS:
		return symbolic.NewBinaryOperation(left, right, symbolic.LT)
	case token.LEQ:
		return symbolic.NewBinaryOperation(left, right, symbolic.LE)
	case token.GTR:
		return symbolic.NewBinaryOperation(left, right, symbolic.GT)
	case token.GEQ:
		return symbolic.NewBinaryOperation(left, right, symbolic.GE)
	case token.LAND:
		return symbolic.NewLogicalOperation([]symbolic.SymbolicExpression{left, right}, symbolic.AND)
	case token.LOR:
		return symbolic.NewLogicalOperation([]symbolic.SymbolicExpression{left, right}, symbolic.OR)
	case token.AND, token.OR, token.XOR, token.SHL, token.SHR, token.AND_NOT:
		// Побитовые операции — грубое приближение: свежая символьная переменная
		return symbolic.NewSymbolicVariable(name, symbolic.IntType)
	default:
		panic(fmt.Sprintf("неподдерживаемый бинарный оператор: %s", op))
	}
}

// foldConstants вычисляет бинарную операцию над двумя конкретными константами.
// Возвращает (результат, true), если свёртка удалась.
func foldConstants(op token.Token, left, right symbolic.SymbolicExpression) (symbolic.SymbolicExpression, bool) {
	if leftInt, ok := left.(*symbolic.IntConstant); ok {
		if rightInt, ok := right.(*symbolic.IntConstant); ok {
			l, r := leftInt.Value, rightInt.Value
			switch op {
			case token.ADD:
				return symbolic.NewIntConstant(l + r), true
			case token.SUB:
				return symbolic.NewIntConstant(l - r), true
			case token.MUL:
				return symbolic.NewIntConstant(l * r), true
			case token.QUO:
				if r != 0 {
					return symbolic.NewIntConstant(l / r), true
				}
			case token.REM:
				if r != 0 {
					return symbolic.NewIntConstant(l % r), true
				}
			case token.EQL:
				return symbolic.NewBoolConstant(l == r), true
			case token.NEQ:
				return symbolic.NewBoolConstant(l != r), true
			case token.LSS:
				return symbolic.NewBoolConstant(l < r), true
			case token.LEQ:
				return symbolic.NewBoolConstant(l <= r), true
			case token.GTR:
				return symbolic.NewBoolConstant(l > r), true
			case token.GEQ:
				return symbolic.NewBoolConstant(l >= r), true
			}
		}
	}
	if leftBool, ok := left.(*symbolic.BoolConstant); ok {
		if rightBool, ok := right.(*symbolic.BoolConstant); ok {
			l, r := leftBool.Value, rightBool.Value
			switch op {
			case token.EQL:
				return symbolic.NewBoolConstant(l == r), true
			case token.NEQ:
				return symbolic.NewBoolConstant(l != r), true
			case token.LAND:
				return symbolic.NewBoolConstant(l && r), true
			case token.LOR:
				return symbolic.NewBoolConstant(l || r), true
			}
		}
	}
	return nil, false
}

// computeUnaryOp отображает токен унарной операции на символьный оператор
func computeUnaryOp(op token.Token, operand symbolic.SymbolicExpression) symbolic.SymbolicExpression {
	switch op {
	case token.SUB:
		if intConst, ok := operand.(*symbolic.IntConstant); ok {
			return symbolic.NewIntConstant(-intConst.Value)
		}
		return symbolic.NewUnaryOperation(operand, symbolic.UMINUS)
	case token.NOT:
		if boolConst, ok := operand.(*symbolic.BoolConstant); ok {
			return symbolic.NewBoolConstant(!boolConst.Value)
		}
		return symbolic.NewUnaryOperation(operand, symbolic.LNOT)
	case token.ADD:
		// Унарный плюс не меняет значение
		return operand
	case token.XOR:
		// Побитовое отрицание ^x в дополнительном коде тождественно -x - 1
		return symbolic.NewBinaryOperation(
			symbolic.NewUnaryOperation(operand, symbolic.UMINUS),
			symbolic.NewIntConstant(1),
			symbolic.SUB,
		)
	default:
		panic(fmt.Sprintf("неподдерживаемый унарный оператор: %s", op))
	}
}

// constToSymbolic переводит ssa-константу в символьную константу
func constToSymbolic(c *ssa.Const) symbolic.SymbolicExpression {
	if c.Value == nil {
		panic(fmt.Sprintf("nil-константы не поддерживаются: %s", c))
	}
	if c.Value.Kind() == constant.Bool {
		return symbolic.NewBoolConstant(constant.BoolVal(c.Value))
	}
	if intVal, ok := constant.Int64Val(c.Value); ok {
		return symbolic.NewIntConstant(intVal)
	}
	panic(fmt.Sprintf("неподдерживаемая константа: %s (поддерживаются только int и bool)", c.Value))
}

// extendPathCondition добавляет условие к path condition через логическое И,
// опуская тривиальную константу true
func extendPathCondition(pc, cond symbolic.SymbolicExpression) symbolic.SymbolicExpression {
	if boolConst, ok := pc.(*symbolic.BoolConstant); ok && boolConst.Value {
		return cond
	}
	return symbolic.NewLogicalOperation([]symbolic.SymbolicExpression{pc, cond}, symbolic.AND)
}

// negate строит отрицание символьного условия
func negate(expr symbolic.SymbolicExpression) symbolic.SymbolicExpression {
	if boolConst, ok := expr.(*symbolic.BoolConstant); ok {
		return symbolic.NewBoolConstant(!boolConst.Value)
	}
	return symbolic.NewLogicalOperation([]symbolic.SymbolicExpression{expr}, symbolic.NOT)
}

// currentFrame возвращает верхний кадр стека вызовов
func (interpreter *Interpreter) currentFrame() *CallStackFrame {
	return &interpreter.CallStack[len(interpreter.CallStack)-1]
}

// advance переходит к следующей инструкции текущего блока
func (interpreter *Interpreter) advance() []Interpreter {
	interpreter.currentFrame().InstrIndex++
	return []Interpreter{*interpreter}
}

// goToBlock переводит исполнение в начало заданного базового блока
func (interpreter *Interpreter) goToBlock(block *ssa.BasicBlock) {
	frame := interpreter.currentFrame()
	frame.PrevBlock = frame.CurrentBlock
	frame.CurrentBlock = block
	frame.InstrIndex = 0
}

// clone создаёт независимую копию состояния интерпретатора
// (глубокое копирование стека вызовов и локальной памяти)
func (interpreter *Interpreter) clone() *Interpreter {
	newStack := make([]CallStackFrame, len(interpreter.CallStack))
	for i, frame := range interpreter.CallStack {
		newLocals := make(map[string]symbolic.SymbolicExpression, len(frame.LocalMemory))
		for name, expr := range frame.LocalMemory {
			newLocals[name] = expr
		}
		newStack[i] = CallStackFrame{
			Function:     frame.Function,
			LocalMemory:  newLocals,
			ReturnValue:  frame.ReturnValue,
			CurrentBlock: frame.CurrentBlock,
			PrevBlock:    frame.PrevBlock,
			InstrIndex:   frame.InstrIndex,
		}
	}
	return &Interpreter{
		CallStack:     newStack,
		Analyser:      interpreter.Analyser,
		PathCondition: interpreter.PathCondition,
		Heap:          interpreter.Heap,
	}
}

// String возвращает строковое представление финального состояния интерпретатора
func (interpreter *Interpreter) String() string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("Path condition: %s\n", interpreter.PathCondition))

	frame := interpreter.currentFrame()
	if frame.ReturnValue != nil {
		builder.WriteString(fmt.Sprintf("Return value: %s\n", frame.ReturnValue.String()))
	} else {
		builder.WriteString("Return value: <none>\n")
	}

	locals := make([]string, 0, len(frame.LocalMemory))
	for name := range frame.LocalMemory {
		locals = append(locals, name)
	}
	sort.Strings(locals)
	builder.WriteString("Locals:\n")
	for _, name := range locals {
		builder.WriteString(fmt.Sprintf("  %s = %s\n", name, frame.LocalMemory[name]))
	}

	return builder.String()
}
