package query

import (
	"github.com/localvar/xuandb/pkg/meta"
	"github.com/localvar/xuandb/pkg/query/parser"
)

type CreateUserStatement parser.CreateUserStatement

func (s *CreateUserStatement) Execute() error {
	u := &meta.User{Name: s.Name, Password: s.Password}
	return meta.CreateUser(u)
}
