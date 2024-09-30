package query

import (
	"github.com/localvar/xuandb/pkg/parser"
	"github.com/localvar/xuandb/pkg/services/metaapi"
)

type CreateUserStatement parser.CreateUserStatement

func (s *CreateUserStatement) Execute() error {
	u := metaapi.User{Name: s.Name, Password: s.Password}
	return metaapi.AddUser(u)
}
