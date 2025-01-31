package common

import (
	"go/ast"
	"reflect"

	log "github.com/sirupsen/logrus"
)

func ShouldSucc(err error) {
	if err != nil {
		log.Panic(err.Error())
	}
}

func IsExportedOrBuiltin(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}
