package translator

import (
	"testing"

	"github.com/ebukreev/go-z3/z3"
	"symbolic-execution-course/internal/symbolic"
)

// newSolverOnTranslator создаёт Z3 solver на контексте транслятора
func newSolverOnTranslator(zt *Z3Translator) *z3.Solver {
	return z3.NewSolver(zt.GetContext().(*z3.Context))
}

func TestTranslateVariable(t *testing.T) {
	zt := NewZ3Translator()

	x := symbolic.NewSymbolicVariable("x", symbolic.IntType)
	result, err := zt.TranslateExpression(x)
	if err != nil {
		t.Fatalf("Translation error: %v", err)
	}
	if _, ok := result.(z3.Int); !ok {
		t.Errorf("Expected z3.Int, got %T", result)
	}

	flag := symbolic.NewSymbolicVariable("flag", symbolic.BoolType)
	result, err = zt.TranslateExpression(flag)
	if err != nil {
		t.Fatalf("Translation error: %v", err)
	}
	if _, ok := result.(z3.Bool); !ok {
		t.Errorf("Expected z3.Bool, got %T", result)
	}
}

func TestVariableCaching(t *testing.T) {
	zt := NewZ3Translator()

	x := symbolic.NewSymbolicVariable("x", symbolic.IntType)
	y := symbolic.NewSymbolicVariable("y", symbolic.IntType)

	if _, err := zt.TranslateExpression(x); err != nil {
		t.Fatalf("Translation error: %v", err)
	}
	if len(zt.vars) != 1 {
		t.Fatalf("Expected 1 cached variable, got %d", len(zt.vars))
	}
	if _, exists := zt.vars["x"]; !exists {
		t.Error("Expected 'x' in variable cache")
	}

	if _, err := zt.TranslateExpression(y); err != nil {
		t.Fatalf("Translation error: %v", err)
	}
	if len(zt.vars) != 2 {
		t.Errorf("Expected 2 cached variables, got %d", len(zt.vars))
	}

	// Повторная трансляция 'x' не должна добавлять новую запись в кэш
	if _, err := zt.TranslateExpression(symbolic.NewSymbolicVariable("x", symbolic.IntType)); err != nil {
		t.Fatalf("Translation error: %v", err)
	}
	if len(zt.vars) != 2 {
		t.Errorf("Expected cache to stay at 2 variables, got %d", len(zt.vars))
	}
}

func TestTranslateConstants(t *testing.T) {
	zt := NewZ3Translator()

	five, err := zt.TranslateExpression(symbolic.NewIntConstant(5))
	if err != nil {
		t.Fatalf("Translation error: %v", err)
	}
	v, isLit, ok := five.(z3.Int).AsInt64()
	if !ok || !isLit || v != 5 {
		t.Errorf("Expected literal 5, got %v (isLit=%v, ok=%v)", v, isLit, ok)
	}

	tr, err := zt.TranslateExpression(symbolic.NewBoolConstant(true))
	if err != nil {
		t.Fatalf("Translation error: %v", err)
	}
	bv, isConst := tr.(z3.Bool).AsBool()
	if !isConst || !bv {
		t.Errorf("Expected bool constant true, got %v (isConst=%v)", bv, isConst)
	}
}

func TestTranslateArithmeticAndComparison(t *testing.T) {
	zt := NewZ3Translator()

	// x + y > 5
	x := symbolic.NewSymbolicVariable("x", symbolic.IntType)
	y := symbolic.NewSymbolicVariable("y", symbolic.IntType)
	five := symbolic.NewIntConstant(5)
	sum := symbolic.NewBinaryOperation(x, y, symbolic.ADD)
	condition := symbolic.NewBinaryOperation(sum, five, symbolic.GT)

	result, err := zt.TranslateExpression(condition)
	if err != nil {
		t.Fatalf("Translation error: %v", err)
	}
	z3Cond, okType := result.(z3.Bool)
	if !okType {
		t.Fatalf("Expected z3.Bool, got %T", result)
	}

	// Проверяем выполнимость: x + y > 5 должно быть SAT
	solver := newSolverOnTranslator(zt)
	solver.Assert(z3Cond)
	sat, err := solver.Check()
	if err != nil {
		t.Fatalf("Solver error: %v", err)
	}
	if !sat {
		t.Error("Expected x + y > 5 to be satisfiable")
	}
}

func TestTranslateAllArithmeticOperators(t *testing.T) {
	zt := NewZ3Translator()

	x := symbolic.NewSymbolicVariable("x", symbolic.IntType)
	y := symbolic.NewSymbolicVariable("y", symbolic.IntType)

	ops := []symbolic.BinaryOperator{symbolic.ADD, symbolic.SUB, symbolic.MUL, symbolic.DIV, symbolic.MOD}
	for _, op := range ops {
		expr := symbolic.NewBinaryOperation(x, y, op)
		result, err := zt.TranslateExpression(expr)
		if err != nil {
			t.Fatalf("Translation error for %s: %v", op, err)
		}
		if _, ok := result.(z3.Int); !ok {
			t.Errorf("Operation %s: expected z3.Int, got %T", op, result)
		}
	}
}

func TestTranslateAllComparisonOperators(t *testing.T) {
	zt := NewZ3Translator()

	x := symbolic.NewSymbolicVariable("x", symbolic.IntType)
	y := symbolic.NewSymbolicVariable("y", symbolic.IntType)

	ops := []symbolic.BinaryOperator{symbolic.EQ, symbolic.NE, symbolic.LT, symbolic.LE, symbolic.GT, symbolic.GE}
	for _, op := range ops {
		expr := symbolic.NewBinaryOperation(x, y, op)
		result, err := zt.TranslateExpression(expr)
		if err != nil {
			t.Fatalf("Translation error for %s: %v", op, err)
		}
		if _, ok := result.(z3.Bool); !ok {
			t.Errorf("Operation %s: expected z3.Bool, got %T", op, result)
		}
	}
}

func TestTranslateLogicalAnd(t *testing.T) {
	zt := NewZ3Translator()

	x := symbolic.NewSymbolicVariable("x", symbolic.IntType)
	y := symbolic.NewSymbolicVariable("y", symbolic.IntType)
	zero := symbolic.NewIntConstant(0)
	ten := symbolic.NewIntConstant(10)

	cond1 := symbolic.NewBinaryOperation(x, zero, symbolic.GT)
	cond2 := symbolic.NewBinaryOperation(y, ten, symbolic.LT)
	andExpr := symbolic.NewLogicalOperation([]symbolic.SymbolicExpression{cond1, cond2}, symbolic.AND)

	result, err := zt.TranslateExpression(andExpr)
	if err != nil {
		t.Fatalf("Translation error: %v", err)
	}
	z3And, okType := result.(z3.Bool)
	if !okType {
		t.Fatalf("Expected z3.Bool, got %T", result)
	}

	solver := newSolverOnTranslator(zt)
	solver.Assert(z3And)
	sat, err := solver.Check()
	if err != nil {
		t.Fatalf("Solver error: %v", err)
	}
	if !sat {
		t.Error("Expected (x > 0) && (y < 10) to be satisfiable")
	}

	// Проверяем модель: x > 0, y < 10
	model := solver.Model()
	xVal := zt.vars["x"].(z3.Int)
	yVal := zt.vars["y"].(z3.Int)
	xRes := model.Eval(xVal, true)
	yRes := model.Eval(yVal, true)

	xInt, _, xOk := xRes.(z3.Int).AsInt64()
	yInt, _, yOk := yRes.(z3.Int).AsInt64()
	if !xOk || !yOk {
		t.Fatalf("Failed to get model values: xOk=%v, yOk=%v", xOk, yOk)
	}
	if xInt <= 0 {
		t.Errorf("Expected x > 0 from model, got %d", xInt)
	}
	if yInt >= 10 {
		t.Errorf("Expected y < 10 from model, got %d", yInt)
	}
}

func TestTranslateLogicalOrAndNot(t *testing.T) {
	zt := NewZ3Translator()

	a := symbolic.NewSymbolicVariable("a", symbolic.BoolType)
	b := symbolic.NewSymbolicVariable("b", symbolic.BoolType)

	orExpr := symbolic.NewLogicalOperation([]symbolic.SymbolicExpression{a, b}, symbolic.OR)
	result, err := zt.TranslateExpression(orExpr)
	if err != nil {
		t.Fatalf("Translation error: %v", err)
	}
	if _, ok := result.(z3.Bool); !ok {
		t.Errorf("Expected z3.Bool, got %T", result)
	}

	notExpr := symbolic.NewLogicalOperation([]symbolic.SymbolicExpression{a}, symbolic.NOT)
	result, err = zt.TranslateExpression(notExpr)
	if err != nil {
		t.Fatalf("Translation error: %v", err)
	}
	if _, ok := result.(z3.Bool); !ok {
		t.Errorf("Expected z3.Bool, got %T", result)
	}
}

func TestTranslateImplies(t *testing.T) {
	zt := NewZ3Translator()

	x := symbolic.NewSymbolicVariable("x", symbolic.IntType)
	zero := symbolic.NewIntConstant(0)
	ten := symbolic.NewIntConstant(10)

	antecedent := symbolic.NewBinaryOperation(x, zero, symbolic.GT)
	consequent := symbolic.NewBinaryOperation(x, ten, symbolic.LT)
	implies := symbolic.NewLogicalOperation(
		[]symbolic.SymbolicExpression{antecedent, consequent}, symbolic.IMPLIES)

	result, err := zt.TranslateExpression(implies)
	if err != nil {
		t.Fatalf("Translation error: %v", err)
	}

	solver := newSolverOnTranslator(zt)
	solver.Assert(result.(z3.Bool))
	sat, err := solver.Check()
	if err != nil {
		t.Fatalf("Solver error: %v", err)
	}
	if !sat {
		t.Error("Expected (x > 0) => (x < 10) to be satisfiable")
	}
}

func TestTranslateUnary(t *testing.T) {
	zt := NewZ3Translator()

	x := symbolic.NewSymbolicVariable("x", symbolic.IntType)
	neg := symbolic.NewUnaryOperation(x, symbolic.UMINUS)
	result, err := zt.TranslateExpression(neg)
	if err != nil {
		t.Fatalf("Translation error: %v", err)
	}
	if _, ok := result.(z3.Int); !ok {
		t.Errorf("Expected z3.Int, got %T", result)
	}

	flag := symbolic.NewSymbolicVariable("flag", symbolic.BoolType)
	notFlag := symbolic.NewUnaryOperation(flag, symbolic.LNOT)
	result, err = zt.TranslateExpression(notFlag)
	if err != nil {
		t.Fatalf("Translation error: %v", err)
	}
	if _, ok := result.(z3.Bool); !ok {
		t.Errorf("Expected z3.Bool, got %T", result)
	}
}

func TestTranslateContradictionUnsat(t *testing.T) {
	zt := NewZ3Translator()

	// x > 0 && !(x > 0) - противоречие
	x := symbolic.NewSymbolicVariable("x", symbolic.IntType)
	zero := symbolic.NewIntConstant(0)
	cond := symbolic.NewBinaryOperation(x, zero, symbolic.GT)
	notCond := symbolic.NewLogicalOperation([]symbolic.SymbolicExpression{cond}, symbolic.NOT)
	contradiction := symbolic.NewLogicalOperation(
		[]symbolic.SymbolicExpression{cond, notCond}, symbolic.AND)

	result, err := zt.TranslateExpression(contradiction)
	if err != nil {
		t.Fatalf("Translation error: %v", err)
	}

	solver := newSolverOnTranslator(zt)
	solver.Assert(result.(z3.Bool))
	sat, err := solver.Check()
	if err != nil {
		t.Fatalf("Solver error: %v", err)
	}
	if sat {
		t.Error("Expected UNSAT for contradiction x > 0 && !(x > 0)")
	}
}

func TestTranslateModArithmetic(t *testing.T) {
	zt := NewZ3Translator()

	// (x % 2 == 0) && (y % 3 == 1)
	x := symbolic.NewSymbolicVariable("x", symbolic.IntType)
	y := symbolic.NewSymbolicVariable("y", symbolic.IntType)
	zero := symbolic.NewIntConstant(0)
	one := symbolic.NewIntConstant(1)
	two := symbolic.NewIntConstant(2)
	three := symbolic.NewIntConstant(3)

	eq1 := symbolic.NewBinaryOperation(
		symbolic.NewBinaryOperation(x, two, symbolic.MOD), zero, symbolic.EQ)
	eq2 := symbolic.NewBinaryOperation(
		symbolic.NewBinaryOperation(y, three, symbolic.MOD), one, symbolic.EQ)
	modExpr := symbolic.NewLogicalOperation(
		[]symbolic.SymbolicExpression{eq1, eq2}, symbolic.AND)

	result, err := zt.TranslateExpression(modExpr)
	if err != nil {
		t.Fatalf("Translation error: %v", err)
	}

	solver := newSolverOnTranslator(zt)
	solver.Assert(result.(z3.Bool))
	sat, err := solver.Check()
	if err != nil {
		t.Fatalf("Solver error: %v", err)
	}
	if !sat {
		t.Error("Expected (x % 2 == 0) && (y % 3 == 1) to be satisfiable")
	}
}

func TestTranslateRef(t *testing.T) {
	zt := NewZ3Translator()

	// Ссылка кодируется целочисленной Z3-константой (адресом объекта)
	ref := symbolic.NewRef(1, symbolic.StructType)
	result, err := zt.TranslateExpression(ref)
	if err != nil {
		t.Fatalf("Translation error: %v", err)
	}
	if _, ok := result.(z3.Int); !ok {
		t.Errorf("Expected z3.Int, got %T", result)
	}

	// Повторная трансляция той же ссылки должна вернуть ту же константу (кэш)
	again, err := zt.TranslateExpression(symbolic.NewRef(1, symbolic.StructType))
	if err != nil {
		t.Fatalf("Translation error: %v", err)
	}
	if _, ok := zt.vars["ref#1"]; !ok {
		t.Error("Expected 'ref#1' in variable cache")
	}
	if result.(z3.Int).String() != again.(z3.Int).String() {
		t.Errorf("Expected cached ref constant, got %s vs %s", result, again)
	}
}

func TestTranslateArrayStoreSelect(t *testing.T) {
	zt := NewZ3Translator()

	// store(heap, 5, 10) и последующее чтение select(..., 5) должно давать 10
	heap := symbolic.NewArrayVariable("heap", symbolic.IntArraySort())
	stored := symbolic.NewArrayStore(heap, symbolic.NewIntConstant(5), symbolic.NewIntConstant(10))
	selected := symbolic.NewArraySelect(stored, symbolic.NewIntConstant(5), symbolic.IntType)

	eqTen := symbolic.NewBinaryOperation(selected, symbolic.NewIntConstant(10), symbolic.EQ)
	result, err := zt.TranslateExpression(eqTen)
	if err != nil {
		t.Fatalf("Translation error: %v", err)
	}

	solver := newSolverOnTranslator(zt)
	solver.Assert(result.(z3.Bool))
	sat, err := solver.Check()
	if err != nil {
		t.Fatalf("Solver error: %v", err)
	}
	if !sat {
		t.Error("Expected select(store(heap, 5, 10), 5) == 10 to be satisfiable")
	}

	// Чтение по другому индексу не обязано давать 10: select(store(heap, 5, 10), 7) != 10 - SAT
	other := symbolic.NewArraySelect(stored, symbolic.NewIntConstant(7), symbolic.IntType)
	neTen := symbolic.NewBinaryOperation(other, symbolic.NewIntConstant(10), symbolic.NE)
	result, err = zt.TranslateExpression(neTen)
	if err != nil {
		t.Fatalf("Translation error: %v", err)
	}

	solver2 := newSolverOnTranslator(zt)
	solver2.Assert(result.(z3.Bool))
	sat, err = solver2.Check()
	if err != nil {
		t.Fatalf("Solver error: %v", err)
	}
	if !sat {
		t.Error("Expected select(store(heap, 5, 10), 7) != 10 to be satisfiable")
	}
}

func TestResetClearsCache(t *testing.T) {
	zt := NewZ3Translator()

	x := symbolic.NewSymbolicVariable("x", symbolic.IntType)
	if _, err := zt.TranslateExpression(x); err != nil {
		t.Fatalf("Translation error: %v", err)
	}
	if len(zt.vars) != 1 {
		t.Fatalf("Expected 1 cached variable, got %d", len(zt.vars))
	}

	zt.Reset()
	if len(zt.vars) != 0 {
		t.Errorf("Expected empty cache after Reset, got %d", len(zt.vars))
	}
}
