package parser

type Statement interface {
}

// CreateUserStatement represents a command for creating a new user.
type CreateUserStatement struct {
	// Name of the user to be created.
	Name string

	// User's password.
	Password string
}

type Expr interface {
}

type IntExpr struct {
	Value uint64
}

type FloatExpr struct {
	Value float64
}

type StringExpr struct {
	Value string
}

type AddExpr struct {
	Left  Expr
	Right Expr
}
