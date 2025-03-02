package services

import (
	"context"
	"github.com/TeaOSLab/EdgeAPI/internal/db/models/authority"
	"github.com/TeaOSLab/EdgeAPI/internal/errors"
	rpcutils "github.com/TeaOSLab/EdgeAPI/internal/rpc/utils"
	"github.com/TeaOSLab/EdgeCommon/pkg/rpc/pb"
	"google.golang.org/grpc/metadata"
)

type AuthorityNodeService struct {
	BaseService
}

// CreateAuthorityNode 创建认证节点
func (this *AuthorityNodeService) CreateAuthorityNode(ctx context.Context, req *pb.CreateAuthorityNodeRequest) (*pb.CreateAuthorityNodeResponse, error) {
	_, _, err := rpcutils.ValidateRequest(ctx, rpcutils.UserTypeAdmin)
	if err != nil {
		return nil, err
	}

	tx := this.NullTx()

	nodeId, err := authority.SharedAuthorityNodeDAO.CreateAuthorityNode(tx, req.Name, req.Description, req.IsOn)
	if err != nil {
		return nil, err
	}

	return &pb.CreateAuthorityNodeResponse{NodeId: nodeId}, nil
}

// UpdateAuthorityNode 修改认证节点
func (this *AuthorityNodeService) UpdateAuthorityNode(ctx context.Context, req *pb.UpdateAuthorityNodeRequest) (*pb.RPCSuccess, error) {
	_, _, err := rpcutils.ValidateRequest(ctx, rpcutils.UserTypeAdmin)
	if err != nil {
		return nil, err
	}

	tx := this.NullTx()

	err = authority.SharedAuthorityNodeDAO.UpdateAuthorityNode(tx, req.NodeId, req.Name, req.Description, req.IsOn)
	if err != nil {
		return nil, err
	}

	return this.Success()
}

// DeleteAuthorityNode 删除认证节点
func (this *AuthorityNodeService) DeleteAuthorityNode(ctx context.Context, req *pb.DeleteAuthorityNodeRequest) (*pb.RPCSuccess, error) {
	_, _, err := rpcutils.ValidateRequest(ctx, rpcutils.UserTypeAdmin)
	if err != nil {
		return nil, err
	}

	tx := this.NullTx()

	err = authority.SharedAuthorityNodeDAO.DisableAuthorityNode(tx, req.NodeId)
	if err != nil {
		return nil, err
	}

	return this.Success()
}

// FindAllEnabledAuthorityNodes 列出所有可用认证节点
func (this *AuthorityNodeService) FindAllEnabledAuthorityNodes(ctx context.Context, req *pb.FindAllEnabledAuthorityNodesRequest) (*pb.FindAllEnabledAuthorityNodesResponse, error) {
	_, _, err := rpcutils.ValidateRequest(ctx, rpcutils.UserTypeAdmin)
	if err != nil {
		return nil, err
	}

	tx := this.NullTx()

	nodes, err := authority.SharedAuthorityNodeDAO.FindAllEnabledAuthorityNodes(tx)
	if err != nil {
		return nil, err
	}

	result := []*pb.AuthorityNode{}
	for _, node := range nodes {
		result = append(result, &pb.AuthorityNode{
			Id:          int64(node.Id),
			IsOn:        node.IsOn == 1,
			UniqueId:    node.UniqueId,
			Secret:      node.Secret,
			Name:        node.Name,
			Description: node.Description,
		})
	}

	return &pb.FindAllEnabledAuthorityNodesResponse{Nodes: result}, nil
}

// CountAllEnabledAuthorityNodes 计算认证节点数量
func (this *AuthorityNodeService) CountAllEnabledAuthorityNodes(ctx context.Context, req *pb.CountAllEnabledAuthorityNodesRequest) (*pb.RPCCountResponse, error) {
	_, _, err := rpcutils.ValidateRequest(ctx, rpcutils.UserTypeAdmin)
	if err != nil {
		return nil, err
	}

	tx := this.NullTx()

	count, err := authority.SharedAuthorityNodeDAO.CountAllEnabledAuthorityNodes(tx)
	if err != nil {
		return nil, err
	}

	return this.SuccessCount(count)
}

// ListEnabledAuthorityNodes 列出单页的认证节点
func (this *AuthorityNodeService) ListEnabledAuthorityNodes(ctx context.Context, req *pb.ListEnabledAuthorityNodesRequest) (*pb.ListEnabledAuthorityNodesResponse, error) {
	_, _, err := rpcutils.ValidateRequest(ctx, rpcutils.UserTypeAdmin)
	if err != nil {
		return nil, err
	}

	tx := this.NullTx()

	nodes, err := authority.SharedAuthorityNodeDAO.ListEnabledAuthorityNodes(tx, req.Offset, req.Size)
	if err != nil {
		return nil, err
	}

	result := []*pb.AuthorityNode{}
	for _, node := range nodes {
		result = append(result, &pb.AuthorityNode{
			Id:          int64(node.Id),
			IsOn:        node.IsOn == 1,
			UniqueId:    node.UniqueId,
			Secret:      node.Secret,
			Name:        node.Name,
			Description: node.Description,
			StatusJSON:  []byte(node.Status),
		})
	}

	return &pb.ListEnabledAuthorityNodesResponse{Nodes: result}, nil
}

// FindEnabledAuthorityNode 根据ID查找节点
func (this *AuthorityNodeService) FindEnabledAuthorityNode(ctx context.Context, req *pb.FindEnabledAuthorityNodeRequest) (*pb.FindEnabledAuthorityNodeResponse, error) {
	_, _, err := rpcutils.ValidateRequest(ctx, rpcutils.UserTypeAdmin)
	if err != nil {
		return nil, err
	}

	tx := this.NullTx()

	node, err := authority.SharedAuthorityNodeDAO.FindEnabledAuthorityNode(tx, req.NodeId)
	if err != nil {
		return nil, err
	}

	if node == nil {
		return &pb.FindEnabledAuthorityNodeResponse{Node: nil}, nil
	}

	result := &pb.AuthorityNode{
		Id:          int64(node.Id),
		IsOn:        node.IsOn == 1,
		UniqueId:    node.UniqueId,
		Secret:      node.Secret,
		Name:        node.Name,
		Description: node.Description,
	}
	return &pb.FindEnabledAuthorityNodeResponse{Node: result}, nil
}

// FindCurrentAuthorityNode 获取当前认证节点的版本
func (this *AuthorityNodeService) FindCurrentAuthorityNode(ctx context.Context, req *pb.FindCurrentAuthorityNodeRequest) (*pb.FindCurrentAuthorityNodeResponse, error) {
	_, err := this.ValidateAuthority(ctx)
	if err != nil {
		return nil, err
	}

	tx := this.NullTx()

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("context: need 'nodeId'")
	}
	nodeIds := md.Get("nodeid")
	if len(nodeIds) == 0 {
		return nil, errors.New("invalid 'nodeId'")
	}
	nodeId := nodeIds[0]
	node, err := authority.SharedAuthorityNodeDAO.FindEnabledAuthorityNodeWithUniqueId(tx, nodeId)
	if err != nil {
		return nil, err
	}

	if node == nil {
		return &pb.FindCurrentAuthorityNodeResponse{Node: nil}, nil
	}

	result := &pb.AuthorityNode{
		Id:          int64(node.Id),
		IsOn:        node.IsOn == 1,
		UniqueId:    node.UniqueId,
		Secret:      node.Secret,
		Name:        node.Name,
		Description: node.Description,
	}
	return &pb.FindCurrentAuthorityNodeResponse{Node: result}, nil
}

// UpdateAuthorityNodeStatus 更新节点状态
func (this *AuthorityNodeService) UpdateAuthorityNodeStatus(ctx context.Context, req *pb.UpdateAuthorityNodeStatusRequest) (*pb.RPCSuccess, error) {
	// 校验节点
	_, nodeId, err := this.ValidateNodeId(ctx, rpcutils.UserTypeAuthority)
	if err != nil {
		return nil, err
	}

	if req.NodeId > 0 {
		nodeId = req.NodeId
	}

	if nodeId <= 0 {
		return nil, errors.New("'nodeId' should be greater than 0")
	}

	tx := this.NullTx()

	err = authority.SharedAuthorityNodeDAO.UpdateNodeStatus(tx, nodeId, req.StatusJSON)
	if err != nil {
		return nil, err
	}
	return this.Success()
}
