package huaweicloud

import (
	"context"

	ecs "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2"
	ecsmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ecs/v2/model"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"
)

type SecurityGroupListener struct {
	cloudProvider    *CloudProvider
	ecsApiClient     *ecs.EcsClient
	cloudConfigSecId string
}

func (s *SecurityGroupListener) startSecurityGroupListener() error {
	watcher, err := s.cloudProvider.kubeClient.Nodes().Watch(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Errorf("failed to watch kubernetes cluster node list: %s", err)
		return err
	}

	for {
		for event := range watcher.ResultChan() {
			newNode, ok := event.Object.(*v1.Node)
			if !ok {
				klog.Error("unexpected type:%v", event.Type)
				continue
			}
			switch event.Type {
			case watch.Added:
				node, err := s.cloudProvider.Basic.ecsClient.GetByNodeName(newNode.Name)
				if err != nil {
					klog.Error("can not get kubernetes node name: %v", err)
					continue
				}
				if node.Name != newNode.Name {
					klog.Warning("add can not find kubernetes node name corresponding to ECS name")
					continue
				}
				err = s.associateSecurityGroup(node)
				if err != nil {
					klog.Error(err)
					continue
				}
			case watch.Deleted:
				node, err := s.cloudProvider.Basic.ecsClient.GetByNodeName(newNode.Name)
				if err != nil {
					klog.Error("can not get kubernetes node name: %v", err)
					continue
				}
				if node.Name != newNode.Name {
					klog.Warning("delete can not find kubernetes node name corresponding to ECS name")
					continue
				}
				err = s.disassociateSecurityGroup(node)
				if err != nil {
					klog.Error(err)
					continue
				}
			}
		}
	}
}

func (s *SecurityGroupListener) associateSecurityGroup(ecsNode *ecsmodel.ServerDetail) error {
	klog.Info("start to associate security group to ECS")
	isBond := false // default to no bond
	for _, sec := range ecsNode.SecurityGroups {
		if sec.Id == s.cloudConfigSecId {
			isBond = true
			break
		}
	}
	// node already bond cloudConfigSecId
	if isBond {
		return nil
	}

	// associate cloudConfigSecId to ECS
	_, err := s.ecsApiClient.NovaAssociateSecurityGroup(&ecsmodel.NovaAssociateSecurityGroupRequest{
		ServerId: ecsNode.Id,
		Body: &ecsmodel.NovaAssociateSecurityGroupRequestBody{
			AddSecurityGroup: &ecsmodel.NovaAddSecurityGroupOption{
				Name: s.cloudConfigSecId,
			},
		},
	})
	if err != nil {
		klog.Errorf("failed to associate security group %s to ECS %s: %s", s.cloudConfigSecId, ecsNode.Id, err)
		return err
	}
	return nil
}

func (s *SecurityGroupListener) disassociateSecurityGroup(ecsNode *ecsmodel.ServerDetail) error {
	klog.Info("start to disassociate security group from ECS")
	isBond := false // default to no bond
	for _, sec := range ecsNode.SecurityGroups {
		if sec.Id == s.cloudConfigSecId {
			isBond = true
			break
		}
	}
	// node unbind cloudConfigSecId
	if !isBond {
		return nil
	}

	// disassociate cloudConfigSecId from ECS
	_, err := s.ecsApiClient.NovaDisassociateSecurityGroup(&ecsmodel.NovaDisassociateSecurityGroupRequest{
		ServerId: ecsNode.Id,
		Body: &ecsmodel.NovaDisassociateSecurityGroupRequestBody{
			RemoveSecurityGroup: &ecsmodel.NovaRemoveSecurityGroupOption{
				Name: s.cloudConfigSecId,
			},
		},
	})
	if err != nil {
		klog.Errorf("failed to disassociate security group %s from ECS %s: %s", s.cloudConfigSecId, ecsNode.Id, err)
		return err
	}
	return nil
}
