package models

import (
	"errors"
	"github.com/TeaOSLab/EdgeCommon/pkg/nodeconfigs"
	_ "github.com/go-sql-driver/mysql"
	"github.com/iwind/TeaGo/Tea"
	"github.com/iwind/TeaGo/dbs"
	"github.com/iwind/TeaGo/types"
)

const (
	NodeIPAddressStateEnabled  = 1 // 已启用
	NodeIPAddressStateDisabled = 0 // 已禁用
)

type NodeIPAddressDAO dbs.DAO

func NewNodeIPAddressDAO() *NodeIPAddressDAO {
	return dbs.NewDAO(&NodeIPAddressDAO{
		DAOObject: dbs.DAOObject{
			DB:     Tea.Env,
			Table:  "edgeNodeIPAddresses",
			Model:  new(NodeIPAddress),
			PkName: "id",
		},
	}).(*NodeIPAddressDAO)
}

var SharedNodeIPAddressDAO *NodeIPAddressDAO

func init() {
	dbs.OnReady(func() {
		SharedNodeIPAddressDAO = NewNodeIPAddressDAO()
	})
}

// EnableAddress 启用条目
func (this *NodeIPAddressDAO) EnableAddress(tx *dbs.Tx, id int64) (err error) {
	_, err = this.Query(tx).
		Pk(id).
		Set("state", NodeIPAddressStateEnabled).
		Update()
	return err
}

// DisableAddress 禁用IP地址
func (this *NodeIPAddressDAO) DisableAddress(tx *dbs.Tx, id int64) (err error) {
	_, err = this.Query(tx).
		Pk(id).
		Set("state", NodeIPAddressStateDisabled).
		Update()
	return err
}

// DisableAllAddressesWithNodeId 禁用节点的所有的IP地址
func (this *NodeIPAddressDAO) DisableAllAddressesWithNodeId(tx *dbs.Tx, nodeId int64, role nodeconfigs.NodeRole) error {
	if nodeId <= 0 {
		return errors.New("invalid nodeId")
	}
	if len(role) == 0 {
		role = nodeconfigs.NodeRoleNode
	}
	_, err := this.Query(tx).
		Attr("nodeId", nodeId).
		Set("state", NodeIPAddressStateDisabled).
		Update()
	return err
}

// FindEnabledAddress 查找启用中的IP地址
func (this *NodeIPAddressDAO) FindEnabledAddress(tx *dbs.Tx, id int64) (*NodeIPAddress, error) {
	result, err := this.Query(tx).
		Pk(id).
		Attr("state", NodeIPAddressStateEnabled).
		Find()
	if result == nil {
		return nil, err
	}
	return result.(*NodeIPAddress), err
}

// FindAddressName 根据主键查找名称
func (this *NodeIPAddressDAO) FindAddressName(tx *dbs.Tx, id int64) (string, error) {
	return this.Query(tx).
		Pk(id).
		Result("name").
		FindStringCol("")
}

// CreateAddress 创建IP地址
func (this *NodeIPAddressDAO) CreateAddress(tx *dbs.Tx, nodeId int64, role nodeconfigs.NodeRole, name string, ip string, canAccess bool) (addressId int64, err error) {
	if len(role) == 0 {
		role = nodeconfigs.NodeRoleNode
	}

	op := NewNodeIPAddressOperator()
	op.NodeId = nodeId
	op.Role = role
	op.Name = name
	op.Ip = ip
	op.CanAccess = canAccess
	op.State = NodeIPAddressStateEnabled
	err = this.Save(tx, op)
	if err != nil {
		return 0, err
	}

	err = SharedNodeDAO.NotifyDNSUpdate(tx, nodeId)
	if err != nil {
		return 0, err
	}

	return types.Int64(op.Id), nil
}

// UpdateAddress 修改IP地址
func (this *NodeIPAddressDAO) UpdateAddress(tx *dbs.Tx, addressId int64, name string, ip string, canAccess bool) (err error) {
	if addressId <= 0 {
		return errors.New("invalid addressId")
	}

	op := NewNodeIPAddressOperator()
	op.Id = addressId
	op.Name = name
	op.Ip = ip
	op.CanAccess = canAccess
	op.State = NodeIPAddressStateEnabled // 恢复状态
	err = this.Save(tx, op)
	return err
}

// UpdateAddressIP 修改IP地址中的IP
func (this *NodeIPAddressDAO) UpdateAddressIP(tx *dbs.Tx, addressId int64, ip string) error {
	if addressId <= 0 {
		return errors.New("invalid addressId")
	}
	op := NewNodeIPAddressOperator()
	op.Id = addressId
	op.Ip = ip
	err := this.Save(tx, op)
	return err
}

// UpdateAddressNodeId 修改IP地址所属节点
func (this *NodeIPAddressDAO) UpdateAddressNodeId(tx *dbs.Tx, addressId int64, nodeId int64) error {
	_, err := this.Query(tx).
		Pk(addressId).
		Set("nodeId", nodeId).
		Set("state", NodeIPAddressStateEnabled). // 恢复状态
		Update()
	if err != nil {
		return err
	}

	err = SharedNodeDAO.NotifyDNSUpdate(tx, nodeId)
	if err != nil {
		return err
	}
	return nil
}

// FindAllEnabledAddressesWithNode 查找节点的所有的IP地址
func (this *NodeIPAddressDAO) FindAllEnabledAddressesWithNode(tx *dbs.Tx, nodeId int64, role nodeconfigs.NodeRole) (result []*NodeIPAddress, err error) {
	if len(role) == 0 {
		role = nodeconfigs.NodeRoleNode
	}
	_, err = this.Query(tx).
		Attr("nodeId", nodeId).
		Attr("role", role).
		State(NodeIPAddressStateEnabled).
		Desc("order").
		AscPk().
		Slice(&result).
		FindAll()
	return
}

// FindFirstNodeAccessIPAddress 查找节点的第一个可访问的IP地址
func (this *NodeIPAddressDAO) FindFirstNodeAccessIPAddress(tx *dbs.Tx, nodeId int64, role nodeconfigs.NodeRole) (string, error) {
	if len(role) == 0 {
		role = nodeconfigs.NodeRoleNode
	}
	return this.Query(tx).
		Attr("nodeId", nodeId).
		Attr("role", role).
		State(NodeIPAddressStateEnabled).
		Attr("canAccess", true).
		Desc("order").
		AscPk().
		Result("ip").
		FindStringCol("")
}

// FindFirstNodeAccessIPAddressId 查找节点的第一个可访问的IP地址ID
func (this *NodeIPAddressDAO) FindFirstNodeAccessIPAddressId(tx *dbs.Tx, nodeId int64, role nodeconfigs.NodeRole) (int64, error) {
	if len(role) == 0 {
		role = nodeconfigs.NodeRoleNode
	}
	return this.Query(tx).
		Attr("nodeId", nodeId).
		Attr("role", role).
		State(NodeIPAddressStateEnabled).
		Attr("canAccess", true).
		Desc("order").
		AscPk().
		Result("id").
		FindInt64Col(0)
}

// FindNodeAccessIPAddresses 查找节点所有的可访问的IP地址
func (this *NodeIPAddressDAO) FindNodeAccessIPAddresses(tx *dbs.Tx, nodeId int64, role nodeconfigs.NodeRole) (result []*NodeIPAddress, err error) {
	if len(role) == 0 {
		role = nodeconfigs.NodeRoleNode
	}
	_, err = this.Query(tx).
		Attr("nodeId", nodeId).
		State(NodeIPAddressStateEnabled).
		Attr("canAccess", true).
		Desc("order").
		AscPk().
		Slice(&result).
		FindAll()
	return
}
