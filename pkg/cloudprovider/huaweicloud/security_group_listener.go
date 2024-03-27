package huaweicloud

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/cloud-provider-huaweicloud/pkg/cloudprovider/huaweicloud/wrapper"

	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"
)

type SecurityGroupListener struct {
	kubeClient *corev1.CoreV1Client
	ecsClient  *wrapper.EcsClient

	securityGroupID string

	stopChannel chan struct{}
}

func (s *SecurityGroupListener) startSecurityGroupListener() {
	if s.securityGroupID == "" {
		klog.Infof(`"security-group-id" is empty, nodes are added or removed will not be associated or disassociated 
	any security groups`)
		return
	}

	klog.Info("starting SecurityGroupListener")
	for {
		securityGroupInformer, err := s.CreateSecurityGroupInformer()
		if err != nil {
			klog.Errorf("failed to watch kubernetes cluster node list, starting SecurityGroupListener failed: %s", err)
			continue
		}

		go securityGroupInformer.Run(s.stopChannel)
		break
	}
}

func (s *SecurityGroupListener) CreateSecurityGroupInformer() (cache.SharedIndexInformer, error) {
	nodeList, err := s.kubeClient.Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Errorf("failed to query a list of node, try again later, error: %s", err)
		time.Sleep(5 * time.Second)
		return nil, err
	}
	nodeInformer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if options.ResourceVersion == "" || options.ResourceVersion == "0" {
					options.ResourceVersion = nodeList.ResourceVersion
				}
				return s.kubeClient.Nodes().List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if options.ResourceVersion == "" || options.ResourceVersion == "0" {
					options.ResourceVersion = nodeList.ResourceVersion
				}
				return s.kubeClient.Nodes().Watch(context.TODO(), options)
			},
		},
		&v1.Node{},
		0,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)

	_, err = nodeInformer.AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			kubeNode, ok := obj.(*v1.Node)
			if !ok {
				klog.Errorf("detected that a node has been added, but the type conversion failed: %#v", obj)
				return
			}
			klog.Infof("detected that a new node has been added to the cluster(%v): %s", ok, kubeNode.Name)

			ecsNode, err := s.ecsClient.GetByNodeName(kubeNode.Name)
			if err != nil {
				klog.Error("Add node: can not get kubernetes node name: %v", err)
				return
			}
			err = s.ecsClient.AssociateSecurityGroup(ecsNode.Id, s.securityGroupID)
			if err != nil {
				klog.Errorf("failed to associate security group %s to ECS %s: %s", s.securityGroupID, ecsNode.Id, err)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {},
		DeleteFunc: func(obj interface{}) {
			kubeNode, ok := obj.(*v1.Node)
			if !ok {
				klog.Errorf("detected that a node has been removed, but the type conversion failed: %#v", obj)
				return
			}
			klog.Infof("detected that a node has been removed to the cluster: %s", kubeNode.Name)

			ecsNode, err := s.ecsClient.GetByNodeName(kubeNode.Name)
			if err != nil {
				klog.Error("Delete node: can not get kubernetes node name: %v", err)
				return
			}
			err = s.ecsClient.DisassociateSecurityGroup(ecsNode.Id, s.securityGroupID)
			if err != nil {
				klog.Errorf("failed to disassociate security group %s from ECS %s: %s", s.securityGroupID, ecsNode.Id, err)
			}
		},
	}, 5*time.Second)
	if err != nil {
		klog.Errorf("failed to start nodeEventHandler, try again later, error: %s", err)
		time.Sleep(5 * time.Second)
		return nil, err
	}
	return nodeInformer, nil
}

func (s *SecurityGroupListener) stopSecurityGroupListener() {
	klog.Warningf("Stop listening to Security Group")
	s.stopChannel <- struct{}{}
}
