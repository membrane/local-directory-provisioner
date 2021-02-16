/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/golang/glog"
	"io/ioutil"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"net"
	"os"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v6/controller"
	"strconv"
	"strings"
)

const (
	provisionerName    = "predic8.de/local-directory"
	provisionerIDAnn   = "localDirectoryProvisionerIdentity"
	provisionerNameKey = "PROVISIONER_NAME"
	provisionerNodeKey = "predic8.de/nodeName"
)

type cephFSProvisioner struct {
	// Kubernetes Client. Use to retrieve Ceph admin secret
	client kubernetes.Interface
	// Identity of this cephFSProvisioner, generated. Used to identify "this"
	// provisioner's PVs.
	identity string
	nodeName string
}

var _ controller.Qualifier = &cephFSProvisioner{}


func newLocalDirectoryProvisioner(client kubernetes.Interface, id string, nodeName string) controller.Provisioner {
	return &cephFSProvisioner{
		client:          client,
		identity:        id,
		nodeName:        nodeName,
	}
}

var _ controller.Provisioner = &cephFSProvisioner{}

func (p cephFSProvisioner) ShouldProvision(ctx context.Context, claim *v1.PersistentVolumeClaim) bool {
	//// As long as the export limit has not been reached we're ok to provision
	//ok := p.checkExportLimit()
	//if !ok {
	//	glog.Infof("export limit reached. skipping claim %s/%s", claim.Namespace, claim.Name)
	//}
	//return ok
	glog.Infof("shouldProvision(%s)", claim.Name)
	isPlacedOnLocalNode := p.IsPlacedOnLocalNode(claim)
	glog.Infof("isPlacedOnLocalNode = %s", strconv.FormatBool(isPlacedOnLocalNode))
	if isPlacedOnLocalNode {
		return true
	}
	canBePlacedOnLocalNode := p.CanBePlacedOnLocalNode(ctx, claim)
	glog.Infof("canBePlacedOnLocalNode = %s", strconv.FormatBool(canBePlacedOnLocalNode))
	return canBePlacedOnLocalNode
}

func GetMyIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if string(ip) == "127.0.0.1" {
				continue
			}
			return string(ip), nil
		}
	}
	return "", errors.New("no IP found")
}

func (p *cephFSProvisioner) IsPlacedOnLocalNode(claim *v1.PersistentVolumeClaim) (bool) {
	if requestedNodeName, found := claim.Annotations[provisionerNodeKey]; found {
		if requestedNodeName == p.nodeName {
			return true
		}
		return false
	}
	return false
}

func (p *cephFSProvisioner) CanBePlacedOnLocalNode(ctx context.Context, claim *v1.PersistentVolumeClaim) (bool) {
	if requestedNodeName, found := claim.Annotations[provisionerNodeKey]; found {
		if requestedNodeName == p.nodeName {
			return true
		}
		return false
	}


	// check whether there is a claim with the same labels already placed on this node
	var ls bytes.Buffer
	for k, v := range claim.Labels {
		if ls.Len() > 0 {
			ls.WriteString(",")
		}
		ls.WriteString(fmt.Sprintf("%s=%s", k, v))
	}
	if ls.Len() == 0 {
		glog.Infof("Rejecting PVC %s because it has no labels.", claim.Name)
		return false
	}
	glog.Infof("Label Selector: %s", ls.String())
	pvcs, err := p.client.CoreV1().PersistentVolumeClaims(claim.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: ls.String(),
	})
	if err != nil {
		glog.Errorf("Error listing other PVCs while computing canBePlaced for %s: %v", claim.Name, err)
		return false
	}
	for _, pvc := range pvcs.Items {
		if pvc.Name == claim.Name {
			continue
		}
		if requestedNodeName, found2 := pvc.Annotations[provisionerNodeKey]; found2 {
			if requestedNodeName == p.nodeName {
				return false
			}
		}
		for k := range pvc.Labels {
			if _, found3 := claim.Annotations[k]; !found3 {
				continue
			}
		}
	}

	return true
}

func (p *cephFSProvisioner) PlaceOnLocalNode(ctx context.Context, oldClaim *v1.PersistentVolumeClaim) (error) {
	claim := oldClaim.DeepCopy()

	glog.Infof("Placing PVC %s on node %s.", claim.Name, p.nodeName)

	origData, err5 := json.Marshal(oldClaim)
	if err5 != nil {
		return err5
	}

	accessor, err2 := meta.Accessor(claim)
	if err2 != nil {
		return err2
	}

	objLabels := accessor.GetAnnotations()
	if objLabels == nil {
		objLabels = make(map[string]string)
	}
	glog.Infof("current annotations are %v", objLabels)

	//update the pod labels
	newLabels := make(map[string]string)
	//newLabels["policytest2"] = "jeffsays"
	newLabels[provisionerNodeKey] = p.nodeName

	for key, value := range newLabels {
		objLabels[key] = value
	}
	glog.Infof("updated annotations are %v", objLabels)

	accessor.SetAnnotations(objLabels)

	newData, err4 := json.Marshal(claim)
	if err4 != nil {
		return err4
	}

	patchBytes, err6 := strategicpatch.CreateTwoWayMergePatch(origData, newData, oldClaim)
	if err6 != nil {
		return err6
	}
	if len(patchBytes) > 0 {
	}

	glog.Infof("patch bytes: %s", string(patchBytes))

	_, err := p.client.CoreV1().PersistentVolumeClaims(claim.Namespace).Patch(
		ctx,
		claim.Name,
		types.StrategicMergePatchType,
		patchBytes,
		metav1.PatchOptions{})

	return err
}

// Provision creates a storage asset and returns a PV object representing it.
func (p *cephFSProvisioner) Provision(ctx context.Context, options controller.ProvisionOptions) (*v1.PersistentVolume, controller.ProvisioningState, error) {
	glog.Infof("considering PVC %s", options.PVC.Name)

	if options.PVC.Spec.Selector != nil {
		return nil, controller.ProvisioningFinished, fmt.Errorf("claim Selector is not supported")
	}
	baseDir, err := p.parseParameters(options.StorageClass.Parameters)
	if err != nil {
		return nil, controller.ProvisioningFinished, err
	}

	if ! p.IsPlacedOnLocalNode(options.PVC) {
		if ! p.CanBePlacedOnLocalNode(ctx, options.PVC) {
			return nil, controller.ProvisioningFinished, errors.New(fmt.Sprintf("PVC %s cannot be placed on %s.", options.PVC.Name, p.nodeName))
		}
		perr := p.PlaceOnLocalNode(ctx, options.PVC)
		if perr != nil {
			return nil, controller.ProvisioningFinished, perr
		}
	}

	// create random share name
	volumeName := fmt.Sprintf("vol-%s", uuid.NewUUID())
	// provision share
	// create cmd
	path := fmt.Sprintf("%s/%s", baseDir, volumeName)
	mkdirErr := os.Mkdir(path, os.ModePerm)
	if mkdirErr != nil {
		return nil, controller.ProvisioningFinished, mkdirErr
	}

	// validate output

	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: options.PVName,
			Annotations: map[string]string{
				provisionerIDAnn: p.identity,
			},
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: *options.StorageClass.ReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{ //FIXME: kernel cephfs doesn't enforce quota, capacity is not meaningless here.
				v1.ResourceName(v1.ResourceStorage): options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)],
			},
			PersistentVolumeSource: v1.PersistentVolumeSource{
				Local: &v1.LocalVolumeSource{
					Path: path,
				},
			},
			NodeAffinity: &v1.VolumeNodeAffinity{
				Required: &v1.NodeSelector{
					NodeSelectorTerms: [] v1.NodeSelectorTerm {
						{
							MatchExpressions: [] v1.NodeSelectorRequirement{
								{
									Key: "kubernetes.io/hostname",
									Operator: "In",
									Values: []string { p.nodeName },
								},
							},
						},
					},
				},
			},
		},
	}

	glog.Infof("successfully created local dir share %+v", pv.Spec.PersistentVolumeSource.Local)

	return pv, controller.ProvisioningFinished, nil
}

func GetNamespace() (string, error) {
	namespace := os.Getenv("NAMESPACE")
	if len(namespace) != 0 {
		return namespace, nil
	}

	ns, nsErr := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if nsErr != nil {
		return "", nsErr
	}
	return string(ns), nil
}

// Delete removes the storage asset that was created by Provision represented
// by the given PV.
func (p *cephFSProvisioner) Delete(ctx context.Context, volume *v1.PersistentVolume) error {
	ann, ok := volume.Annotations[provisionerIDAnn]
	if !ok {
		return errors.New("identity annotation not found on PV")
	}
	if ann != p.identity {
		return &controller.IgnoredError{Reason: "identity annotation on PV does not match ours"}
	}
	// delete CephFS
	// TODO when beta is removed, have to check kube version and pick v1/beta
	// accordingly: maybe the controller lib should offer a function for that
	//class, err := p.client.StorageV1beta1().StorageClasses().Get(helper.GetPersistentVolumeClass(volume), metav1.GetOptions{})
	//if err != nil {
	//	return err
	//}
	//baseDir, err := p.parseParameters(class.Parameters)
	//if err != nil {
	//	return err
	//}
	// create cmd
	removeErr := os.RemoveAll(volume.Spec.Local.Path)

	if removeErr != nil {
		glog.Errorf("failed to delete dir: %v, output: %v", volume.Spec.Local.Path, removeErr)
		return removeErr
	}


	return nil
}

func (p *cephFSProvisioner) parseParameters(parameters map[string]string) (string, error) {
	var (
		baseDir string
	)

	for k, v := range parameters {
		switch strings.ToLower(k) {
		case "basedir":
			baseDir = v
		default:
			return "", fmt.Errorf("invalid option %q", k)
		}
	}
	// sanity check
	if baseDir == "" {
		return "", fmt.Errorf("missing base dir")
	}
	return baseDir, nil
}
func GetNodeName(ctx context.Context, client kubernetes.Interface) (string, error) {
	nodeName := os.Getenv("NODE_NAME")
	if len(nodeName) > 0 {
		return nodeName, nil
	}

	myIP, err := GetMyIP()
	if err != nil {
		return "", err
	}

	namespace, nsErr :=  GetNamespace()
	if nsErr != nil {
		return "", nsErr
	}

	// check, whether this volume actually belongs onto this node
	podList, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("status.podIP=%s", myIP),
	})
	if err != nil {
		return "", err
	}
	if len(podList.Items) == 0 {
		return "", errors.New("Could not detect NODE_NAME. Either set env var, or fix self-lookup (Pod by IP address).")
	}

	return podList.Items[0].Spec.NodeName, nil
}

var (
	master          = flag.String("master", "", "Master URL")
	kubeconfig      = flag.String("kubeconfig", "", "Absolute path to the kubeconfig")
	id              = flag.String("id", "", "Unique provisioner identity")
)

func main() {
	flag.Parse()
	flag.Set("logtostderr", "true")

	var config *rest.Config
	var err error
	if *master != "" || *kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags(*master, *kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		glog.Fatalf("Failed to create config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Failed to create client: %v", err)
	}

	prName := provisionerName
	prNameFromEnv := os.Getenv(provisionerNameKey)
	if prNameFromEnv != "" {
		prName = prNameFromEnv
	}

	// By default, we use provisioner name as provisioner identity.
	// User may specify their own identity with `-id` flag to distinguish each
	// others, if they deploy more than one CephFS provisioners under same provisioner name.
	prID := prName
	if *id != "" {
		prID = *id
	}

	// The controller needs to know what the server version is because out-of-tree
	// provisioners aren't officially supported until 1.5
	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		glog.Fatalf("Error getting server version: %v", err)
	}

	nodeName, nErr := GetNodeName(context.Background(), clientset)
	if nErr != nil {
		glog.Fatalf("Error getting node name: %v", nErr)
	}

	// Create the provisioner: it implements the Provisioner interface expected by
	// the controller
	glog.Infof("Creating Local Directory provisioner %s with identity: %s", prName, prID)
	cephFSProvisioner := newLocalDirectoryProvisioner(clientset, prID, nodeName)

	// Start the provision controller which will dynamically provision cephFS
	// PVs
	pc := controller.NewProvisionController(
		clientset,
		prName,
		cephFSProvisioner,
		serverVersion.GitVersion,
		controller.LeaderElection(false),
	)

	pc.Run(context.Background())
}
