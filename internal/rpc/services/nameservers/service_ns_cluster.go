// Copyright 2021 Liuxiangchao iwind.liu@gmail.com. All rights reserved.

package nameservers

import (
	"context"
	"github.com/TeaOSLab/EdgeAPI/internal/db/models/nameservers"
	"github.com/TeaOSLab/EdgeAPI/internal/rpc/services"
	"github.com/TeaOSLab/EdgeCommon/pkg/rpc/pb"
)

// NSClusterService 域名服务集群相关服务
type NSClusterService struct {
	services.BaseService
}

// CreateNSCluster 创建集群
func (this *NSClusterService) CreateNSCluster(ctx context.Context, req *pb.CreateNSClusterRequest) (*pb.CreateNSClusterResponse, error) {
	_, err := this.ValidateAdmin(ctx, 0)
	if err != nil {
		return nil, err
	}
	var tx = this.NullTx()
	clusterId, err := nameservers.SharedNSClusterDAO.CreateCluster(tx, req.Name, req.AccessLogJSON)
	if err != nil {
		return nil, err
	}
	return &pb.CreateNSClusterResponse{NsClusterId: clusterId}, nil
}

// UpdateNSCluster 修改集群
func (this *NSClusterService) UpdateNSCluster(ctx context.Context, req *pb.UpdateNSClusterRequest) (*pb.RPCSuccess, error) {
	_, err := this.ValidateAdmin(ctx, 0)
	if err != nil {
		return nil, err
	}
	var tx = this.NullTx()
	err = nameservers.SharedNSClusterDAO.UpdateCluster(tx, req.NsClusterId, req.Name, req.IsOn)
	if err != nil {
		return nil, err
	}
	return this.Success()
}

// FindNSClusterAccessLog 查找集群访问日志配置
func (this *NSClusterService) FindNSClusterAccessLog(ctx context.Context, req *pb.FindNSClusterAccessLogRequest) (*pb.FindNSClusterAccessLogResponse, error) {
	_, err := this.ValidateAdmin(ctx, 0)
	if err != nil {
		return nil, err
	}

	var tx = this.NullTx()
	accessLogJSON, err := nameservers.SharedNSClusterDAO.FindClusterAccessLog(tx, req.NsClusterId)
	if err != nil {
		return nil, err
	}
	return &pb.FindNSClusterAccessLogResponse{AccessLogJSON: accessLogJSON}, nil
}

// UpdateNSClusterAccessLog 修改集群访问日志配置
func (this *NSClusterService) UpdateNSClusterAccessLog(ctx context.Context, req *pb.UpdateNSClusterAccessLogRequest) (*pb.RPCSuccess, error) {
	_, err := this.ValidateAdmin(ctx, 0)
	if err != nil {
		return nil, err
	}

	var tx = this.NullTx()
	err = nameservers.SharedNSClusterDAO.UpdateClusterAccessLog(tx, req.NsClusterId, req.AccessLogJSON)
	if err != nil {
		return nil, err
	}
	return this.Success()
}

// DeleteNSCluster 删除集群
func (this *NSClusterService) DeleteNSCluster(ctx context.Context, req *pb.DeleteNSCluster) (*pb.RPCSuccess, error) {
	_, err := this.ValidateAdmin(ctx, 0)
	if err != nil {
		return nil, err
	}
	var tx = this.NullTx()
	err = nameservers.SharedNSClusterDAO.DisableNSCluster(tx, req.NsClusterId)
	if err != nil {
		return nil, err
	}
	return this.Success()
}

// FindEnabledNSCluster 查找单个可用集群信息
func (this *NSClusterService) FindEnabledNSCluster(ctx context.Context, req *pb.FindEnabledNSClusterRequest) (*pb.FindEnabledNSClusterResponse, error) {
	_, err := this.ValidateAdmin(ctx, 0)
	if err != nil {
		return nil, err
	}
	var tx = this.NullTx()
	cluster, err := nameservers.SharedNSClusterDAO.FindEnabledNSCluster(tx, req.NsClusterId)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return &pb.FindEnabledNSClusterResponse{NsCluster: nil}, nil
	}
	return &pb.FindEnabledNSClusterResponse{NsCluster: &pb.NSCluster{
		Id:         int64(cluster.Id),
		IsOn:       cluster.IsOn == 1,
		Name:       cluster.Name,
		InstallDir: cluster.InstallDir,
	}}, nil
}

// CountAllEnabledNSClusters 计算所有可用集群的数量
func (this *NSClusterService) CountAllEnabledNSClusters(ctx context.Context, req *pb.CountAllEnabledNSClustersRequest) (*pb.RPCCountResponse, error) {
	_, err := this.ValidateAdmin(ctx, 0)
	if err != nil {
		return nil, err
	}
	var tx = this.NullTx()
	count, err := nameservers.SharedNSClusterDAO.CountAllEnabledClusters(tx)
	if err != nil {
		return nil, err
	}
	return this.SuccessCount(count)
}

// ListEnabledNSClusters 列出单页可用集群
func (this *NSClusterService) ListEnabledNSClusters(ctx context.Context, req *pb.ListEnabledNSClustersRequest) (*pb.ListEnabledNSClustersResponse, error) {
	_, err := this.ValidateAdmin(ctx, 0)
	if err != nil {
		return nil, err
	}
	var tx = this.NullTx()
	clusters, err := nameservers.SharedNSClusterDAO.ListEnabledClusters(tx, req.Offset, req.Size)
	if err != nil {
		return nil, err
	}
	var pbClusters = []*pb.NSCluster{}
	for _, cluster := range clusters {
		pbClusters = append(pbClusters, &pb.NSCluster{
			Id:         int64(cluster.Id),
			IsOn:       cluster.IsOn == 1,
			Name:       cluster.Name,
			InstallDir: cluster.InstallDir,
		})
	}
	return &pb.ListEnabledNSClustersResponse{NsClusters: pbClusters}, nil
}

// FindAllEnabledNSClusters 查找所有可用集群
func (this *NSClusterService) FindAllEnabledNSClusters(ctx context.Context, req *pb.FindAllEnabledNSClustersRequest) (*pb.FindAllEnabledNSClustersResponse, error) {
	_, err := this.ValidateAdmin(ctx, 0)
	if err != nil {
		return nil, err
	}
	var tx = this.NullTx()
	clusters, err := nameservers.SharedNSClusterDAO.FindAllEnabledClusters(tx)
	if err != nil {
		return nil, err
	}
	var pbClusters = []*pb.NSCluster{}
	for _, cluster := range clusters {
		pbClusters = append(pbClusters, &pb.NSCluster{
			Id:         int64(cluster.Id),
			IsOn:       cluster.IsOn == 1,
			Name:       cluster.Name,
			InstallDir: cluster.InstallDir,
		})
	}
	return &pb.FindAllEnabledNSClustersResponse{NsClusters: pbClusters}, nil
}
