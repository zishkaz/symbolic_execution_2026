// Package symbolic содержит конкретные реализации символьных выражений
package symbolic

import (
	"fmt"
	"strings"
)

// SymbolicExpression - базовый интерфейс для всех символьных выражений
type SymbolicExpression interface {
	// Type возвращает тип выражения
	Type() ExpressionType

	// String возвращает строковое представление выражения
	String() string

	// Accept принимает visitor для обхода дерева выражений
	Accept(visitor Visitor) interface{}
}

// SymbolicVariable представляет символьную переменную
type SymbolicVariable struct {
	Name     string
	ExprType ExpressionType
}

// NewSymbolicVariable создаёт новую символьную переменную
func NewSymbolicVariable(name string, exprType ExpressionType) *SymbolicVariable {
	return &SymbolicVariable{
		Name:     name,
		ExprType: exprType,
	}
}

// Type возвращает тип переменной
func (sv *SymbolicVariable) Type() ExpressionType {
	return sv.ExprType
}

// String возвращает строковое представление переменной
func (sv *SymbolicVariable) String() string {
	return sv.Name
}

// Accept реализует Visitor pattern
func (sv *SymbolicVariable) Accept(visitor Visitor) interface{} {
	return visitor.VisitVariable(sv)
}

// IntConstant представляет целочисленную константу
type IntConstant struct {
	Value int64
}

// NewIntConstant создаёт новую целочисленную константу
func NewIntConstant(value int64) *IntConstant {
	return &IntConstant{Value: value}
}

// Type возвращает тип константы
func (ic *IntConstant) Type() ExpressionType {
	return IntType
}

// String возвращает строковое представление константы
func (ic *IntConstant) String() string {
	return fmt.Sprintf("%d", ic.Value)
}

// Accept реализует Visitor pattern
func (ic *IntConstant) Accept(visitor Visitor) interface{} {
	return visitor.VisitIntConstant(ic)
}

// BoolConstant представляет булеву константу
type BoolConstant struct {
	Value bool
}

// NewBoolConstant создаёт новую булеву константу
func NewBoolConstant(value bool) *BoolConstant {
	return &BoolConstant{Value: value}
}

// Type возвращает тип константы
func (bc *BoolConstant) Type() ExpressionType {
	return BoolType
}

// String возвращает строковое представление константы
func (bc *BoolConstant) String() string {
	return fmt.Sprintf("%t", bc.Value)
}

// Accept реализует Visitor pattern
func (bc *BoolConstant) Accept(visitor Visitor) interface{} {
	return visitor.VisitBoolConstant(bc)
}

// BinaryOperation представляет бинарную операцию
type BinaryOperation struct {
	Left     SymbolicExpression
	Right    SymbolicExpression
	Operator BinaryOperator
}

// NewBinaryOperation создаёт новую бинарную операцию
func NewBinaryOperation(left, right SymbolicExpression, op BinaryOperator) *BinaryOperation {
	if left == nil || right == nil {
		panic("symbolic: nil operand in binary operation")
	}
	if left.Type() != right.Type() {
		panic(fmt.Sprintf("symbolic: type mismatch in binary operation '%s': %s vs %s",
			op, left.Type(), right.Type()))
	}
	if isArithmeticOperator(op) && left.Type() != IntType {
		panic(fmt.Sprintf("symbolic: arithmetic operation '%s' requires int operands, got %s",
			op, left.Type()))
	}
	return &BinaryOperation{
		Left:     left,
		Right:    right,
		Operator: op,
	}
}

// Type возвращает результирующий тип операции
func (bo *BinaryOperation) Type() ExpressionType {
	if isComparisonOperator(bo.Operator) {
		return BoolType
	}
	return IntType
}

// String возвращает строковое представление операции
func (bo *BinaryOperation) String() string {
	return fmt.Sprintf("(%s %s %s)", bo.Left.String(), bo.Operator, bo.Right.String())
}

// Accept реализует Visitor pattern
func (bo *BinaryOperation) Accept(visitor Visitor) interface{} {
	return visitor.VisitBinaryOperation(bo)
}

// LogicalOperation представляет логическую операцию
type LogicalOperation struct {
	Operands []SymbolicExpression
	Operator LogicalOperator
}

// NewLogicalOperation создаёт новую логическую операцию
func NewLogicalOperation(operands []SymbolicExpression, op LogicalOperator) *LogicalOperation {
	switch op {
	case NOT:
		if len(operands) != 1 {
			panic(fmt.Sprintf("symbolic: NOT requires exactly 1 operand, got %d", len(operands)))
		}
	case IMPLIES:
		if len(operands) != 2 {
			panic(fmt.Sprintf("symbolic: IMPLIES requires exactly 2 operands, got %d", len(operands)))
		}
	default: // AND, OR
		if len(operands) < 2 {
			panic(fmt.Sprintf("symbolic: %s requires at least 2 operands, got %d", op, len(operands)))
		}
	}
	for i, operand := range operands {
		if operand == nil {
			panic(fmt.Sprintf("symbolic: nil operand #%d in logical operation '%s'", i, op))
		}
		if operand.Type() != BoolType {
			panic(fmt.Sprintf("symbolic: logical operation '%s' requires bool operands, operand #%d has type %s",
				op, i, operand.Type()))
		}
	}
	return &LogicalOperation{
		Operands: operands,
		Operator: op,
	}
}

// Type возвращает тип логической операции (всегда bool)
func (lo *LogicalOperation) Type() ExpressionType {
	return BoolType
}

// String возвращает строковое представление логической операции
func (lo *LogicalOperation) String() string {
	if lo.Operator == NOT {
		return fmt.Sprintf("!%s", lo.Operands[0].String())
	}
	if lo.Operator == IMPLIES {
		return fmt.Sprintf("(%s => %s)", lo.Operands[0].String(), lo.Operands[1].String())
	}

	sep := fmt.Sprintf(" %s ", lo.Operator)
	parts := make([]string, len(lo.Operands))
	for i, operand := range lo.Operands {
		parts[i] = operand.String()
	}
	return fmt.Sprintf("(%s)", strings.Join(parts, sep))
}

// Accept реализует Visitor pattern
func (lo *LogicalOperation) Accept(visitor Visitor) interface{} {
	return visitor.VisitLogicalOperation(lo)
}

// isArithmeticOperator проверяет, является ли оператор арифметическим
func isArithmeticOperator(op BinaryOperator) bool {
	switch op {
	case ADD, SUB, MUL, DIV, MOD:
		return true
	default:
		return false
	}
}

// isComparisonOperator проверяет, является ли оператор сравнения
func isComparisonOperator(op BinaryOperator) bool {
	switch op {
	case EQ, NE, LT, LE, GT, GE:
		return true
	default:
		return false
	}
}

// Операторы для бинарных выражений
type BinaryOperator int

const (
	// Арифметические операторы
	ADD BinaryOperator = iota
	SUB
	MUL
	DIV
	MOD

	// Операторы сравнения
	EQ // равно
	NE // не равно
	LT // меньше
	LE // меньше или равно
	GT // больше
	GE // больше или равно
)

// String возвращает строковое представление оператора
func (op BinaryOperator) String() string {
	switch op {
	case ADD:
		return "+"
	case SUB:
		return "-"
	case MUL:
		return "*"
	case DIV:
		return "/"
	case MOD:
		return "%"
	case EQ:
		return "=="
	case NE:
		return "!="
	case LT:
		return "<"
	case LE:
		return "<="
	case GT:
		return ">"
	case GE:
		return ">="
	default:
		return "unknown"
	}
}

// Логические операторы
type LogicalOperator int

const (
	AND LogicalOperator = iota
	OR
	NOT
	IMPLIES
)

// String возвращает строковое представление логического оператора
func (op LogicalOperator) String() string {
	switch op {
	case AND:
		return "&&"
	case OR:
		return "||"
	case NOT:
		return "!"
	case IMPLIES:
		return "=>"
	default:
		return "unknown"
	}
}

// Ref представляет символьную ссылку на область памяти (объект, массив, struct)
type Ref struct {
	ID       int
	ExprType ExpressionType
}

// NewRef создаёт новую символьную ссылку
func NewRef(id int, exprType ExpressionType) *Ref {
	return &Ref{
		ID:       id,
		ExprType: exprType,
	}
}

// Type возвращает тип объекта, на который указывает ссылка
func (ref *Ref) Type() ExpressionType {
	return ref.ExprType
}

// String возвращает строковое представление ссылки
func (ref *Ref) String() string {
	return fmt.Sprintf("ref#%d(%s)", ref.ID, ref.ExprType)
}

// Accept реализует Visitor pattern
func (ref *Ref) Accept(visitor Visitor) interface{} {
	return visitor.VisitRef(ref)
}

// ArrayVariable представляет именованный символьный массив (переменную теории массивов).
// Используется моделью символьной памяти как хранилище полей объектов и элементов массивов.
type ArrayVariable struct {
	Name string
	Sort ArraySort
}

// NewArrayVariable создаёт новый именованный символьный массив заданного сорта
func NewArrayVariable(name string, sort ArraySort) *ArrayVariable {
	if sort.Index != IntType {
		panic(fmt.Sprintf("symbolic: array index type must be int, got %s", sort.Index))
	}
	if sort.Elem == ArrayType && sort.Inner == nil {
		panic(fmt.Sprintf("symbolic: array variable '%s' with array elements requires inner sort", name))
	}
	return &ArrayVariable{
		Name: name,
		Sort: sort,
	}
}

// Type возвращает тип выражения (всегда array)
func (av *ArrayVariable) Type() ExpressionType {
	return ArrayType
}

// String возвращает строковое представление массива
func (av *ArrayVariable) String() string {
	return fmt.Sprintf("%s:%s", av.Name, av.Sort)
}

// Accept реализует Visitor pattern
func (av *ArrayVariable) Accept(visitor Visitor) interface{} {
	return visitor.VisitArrayVariable(av)
}

// ArrayStore представляет функциональное обновление символьного массива:
// результат равен исходному массиву, в котором по индексу записано новое значение
type ArrayStore struct {
	Array SymbolicExpression
	Index SymbolicExpression
	Value SymbolicExpression
}

// NewArrayStore создаёт выражение записи в символьный массив.
// Индексом может быть целочисленное выражение или ссылка (адрес объекта в хипе).
func NewArrayStore(array, index, value SymbolicExpression) *ArrayStore {
	if array == nil || index == nil || value == nil {
		panic("symbolic: nil operand in array store")
	}
	if array.Type() != ArrayType {
		panic(fmt.Sprintf("symbolic: array store requires array operand, got %s", array.Type()))
	}
	if !isIntOrRef(index) {
		panic(fmt.Sprintf("symbolic: array index must be int or ref, got %s", index.Type()))
	}
	return &ArrayStore{
		Array: array,
		Index: index,
		Value: value,
	}
}

// Type возвращает тип выражения (массив, как и у исходного операнда)
func (as *ArrayStore) Type() ExpressionType {
	return ArrayType
}

// String возвращает строковое представление записи в массив
func (as *ArrayStore) String() string {
	return fmt.Sprintf("store(%s, %s, %s)", as.Array.String(), as.Index.String(), as.Value.String())
}

// Accept реализует Visitor pattern
func (as *ArrayStore) Accept(visitor Visitor) interface{} {
	return visitor.VisitArrayStore(as)
}

// ArraySelect представляет чтение элемента символьного массива по индексу
type ArraySelect struct {
	Array      SymbolicExpression
	Index      SymbolicExpression
	ResultType ExpressionType
}

// NewArraySelect создаёт выражение чтения из символьного массива.
// resultType — тип прочитанного значения (IntType, BoolType или ArrayType для вложенных массивов).
func NewArraySelect(array, index SymbolicExpression, resultType ExpressionType) *ArraySelect {
	if array == nil || index == nil {
		panic("symbolic: nil operand in array select")
	}
	if array.Type() != ArrayType {
		panic(fmt.Sprintf("symbolic: array select requires array operand, got %s", array.Type()))
	}
	if !isIntOrRef(index) {
		panic(fmt.Sprintf("symbolic: array index must be int or ref, got %s", index.Type()))
	}
	return &ArraySelect{
		Array:      array,
		Index:      index,
		ResultType: resultType,
	}
}

// isIntOrRef проверяет, что выражение может быть индексом символьного массива:
// целочисленное выражение или ссылка (адрес объекта в хипе)
func isIntOrRef(expr SymbolicExpression) bool {
	if expr.Type() == IntType {
		return true
	}
	_, isRef := expr.(*Ref)
	return isRef
}

// Type возвращает тип прочитанного элемента
func (as *ArraySelect) Type() ExpressionType {
	return as.ResultType
}

// String возвращает строковое представление чтения из массива
func (as *ArraySelect) String() string {
	return fmt.Sprintf("select(%s, %s)", as.Array.String(), as.Index.String())
}

// Accept реализует Visitor pattern
func (as *ArraySelect) Accept(visitor Visitor) interface{} {
	return visitor.VisitArraySelect(as)
}

// UnaryOperator представляет унарный оператор
type UnaryOperator int

const (
	// UMINUS арифметическое отрицание: -x (int)
	UMINUS UnaryOperator = iota
	// LNOT логическое отрицание: !x (bool)
	LNOT
)

// String возвращает строковое представление унарного оператора
func (op UnaryOperator) String() string {
	switch op {
	case UMINUS:
		return "-"
	case LNOT:
		return "!"
	default:
		return "unknown"
	}
}

// UnaryOperation представляет унарную операцию (-x, !x)
type UnaryOperation struct {
	Operand  SymbolicExpression
	Operator UnaryOperator
}

// NewUnaryOperation создаёт новую унарную операцию и проверяет тип операнда
func NewUnaryOperation(operand SymbolicExpression, op UnaryOperator) *UnaryOperation {
	if operand == nil {
		panic("symbolic: nil operand in unary operation")
	}
	switch op {
	case UMINUS:
		if operand.Type() != IntType {
			panic(fmt.Sprintf("symbolic: unary '-' requires int operand, got %s", operand.Type()))
		}
	case LNOT:
		if operand.Type() != BoolType {
			panic(fmt.Sprintf("symbolic: unary '!' requires bool operand, got %s", operand.Type()))
		}
	default:
		panic(fmt.Sprintf("symbolic: unknown unary operator %d", op))
	}
	return &UnaryOperation{
		Operand:  operand,
		Operator: op,
	}
}

// Type возвращает результирующий тип унарной операции
func (uo *UnaryOperation) Type() ExpressionType {
	if uo.Operator == LNOT {
		return BoolType
	}
	return IntType
}

// String возвращает строковое представление унарной операции
func (uo *UnaryOperation) String() string {
	return fmt.Sprintf("%s%s", uo.Operator, uo.Operand.String())
}

// Accept реализует Visitor pattern
func (uo *UnaryOperation) Accept(visitor Visitor) interface{} {
	return visitor.VisitUnaryOperation(uo)
}

// TODO: Добавьте дополнительные типы выражений по необходимости:
// - FunctionCall (вызовы функций: f(x, y))
// - ConditionalExpression (тернарный оператор: condition ? true_expr : false_expr)
