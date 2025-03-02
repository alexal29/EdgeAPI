package models

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/iwind/TeaGo/dbs"
	"testing"
)

func TestNodeDAO_FindAllNodeIdsMatch(t *testing.T) {
	var tx *dbs.Tx
	nodeIds, err := SharedNodeDAO.FindAllNodeIdsMatch(tx, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(nodeIds)
}

func TestNodeDAO_UpdateNodeUp(t *testing.T) {
	dbs.NotifyReady()
	var tx *dbs.Tx
	err := SharedNodeDAO.UpdateNodeUp(tx, 57, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("ok")
}
