%{
package parser
%}

%union {
	stmt    Statement
    str     string
    int     uint64
    float   float64
    bool    bool
}

// Identifiers
// Refer comments of the 'init' function for orders of the tokens below.
%token<str>    IDENT

// Keywords
%token CREATE    USER     WITH     PASSWORD   SELECT     FROM     WHERE
       GROUP     BY       LIMIT    OFFSET     JOIN       BETWEEN  IN
       DATABASE  DURATION ALTER    DROP

// comments
%token<str>    COMMENT

// Value tokens
%token<str>    VAL_STR
%token<int>    VAL_INT  VAL_DURATION
%token<float>  VAL_FLT
%token<bool>   VAL_BOOL

// Operators
%left  OP_ASSIGN
%left  OP_OR
%left  OP_XOR
%left  OP_AND
%right OP_NOT
%left  OP_EQU    OP_NOT_EQU    OP_GT    OP_GTE   OP_LT   OP_LTE
       OP_MATCH  OP_NOT_MATCH
%left  OP_BITWISE_OR
%left  OP_BITWISE_AND
%left  OP_LSHIFT OP_RSHIFT
%left  OP_ADD    OP_SUB
%left  OP_MUL    OP_DIV    OP_MOD
%left  OP_BITWISE_XOR
%right OP_SIGN   OP_BITWISE_NOT

// ERR_TOKEN represents an invalid token, to make test cases work, it must be
// the last token also.
%token<str>    ERR_TOKEN

// Statements
%type<stmt>    STATEMENT CREATE_USER_STATEMENT CREATE_DATABASE_STATEMENT

%%

STATEMENT:
	CREATE_USER_STATEMENT
	{
		$$ = $1
	}
    | CREATE_DATABASE_STATEMENT
    {
        $$ = $1
    }

CREATE_USER_STATEMENT:
    CREATE USER IDENT WITH PASSWORD VAL_STR
    {
        stmt := &CreateUserStatement{}
        stmt.Name = $3
        stmt.Password = $6
        $$ = stmt
    }

CREATE_DATABASE_STATEMENT:
    CREATE DATABASE IDENT WITH DURATION VAL_DURATION
    {
        $$ = nil
    }

%%

// keywords maps of keyword strings to their corresponding IDs, keywords that
// cannot be defined as tokens are put into the map directly.
var keywords = map[string]int {
    "DIV": OP_DIV,
    "MOD": OP_MOD,
    "AND": OP_AND,
    "OR": OP_OR,
    "XOR": OP_XOR,
    "NOT": OP_NOT,
    "LIKE": OP_MATCH,
}

// init add other keywords to the keyword map, to make this possible, IDENT
// must be defined as the last token before the keyword tokens, and COMMENT
// must be defined as the first token after the keyword tokens.
func init() {
    id, idx := 0, 0

    // Find the index of the IDENT token.
    for {
        tok := yyToknames[idx]
        idx++
        if tok == "IDENT" {
            id = IDENT+1
            break
        }
    }

    // Put tokens before the value tokens to the keyword map.
    for idx < len(yyToknames) {
        tok := yyToknames[idx]
        if tok == "COMMENT" {
			break
		}
        keywords[tok] = id
        id++
        idx++
    }
}
