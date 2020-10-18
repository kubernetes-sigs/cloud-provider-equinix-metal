package util

import (
	"bytes"
	"fmt"
	"regexp"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog"
)

// ApplyManifests parse and apply manifests to the kubernetes cluster
func ApplyManifests(m []byte, client kubernetes.Interface) error {
	// read each item in the manifest and deploy it
	// if we could rely on everything being server-side apply, we could
	// just pass it in as a patch, but that really only is supported
	// properly from k8s 1.18.0, so we parse it locally, after splitting the
	// manifests per https://yaml.org/spec/1.2/spec.html

	// save to k8s
	manifests, err := parseK8sYaml(m)
	if err != nil {
		return fmt.Errorf("error parsing manifests: %v", err)
	}
	for i, m := range manifests {
		var err2 error
		klog.V(2).Infof("applying manifest %d %v", i, m.GetObjectKind().GroupVersionKind())
		// now figure out what kind it is
		// for each kind, we get it. There are three possiblities:
		// - got it, no error: Update()
		// - not found, error: Create()
		// - other error: return error
		switch o := m.(type) {
		case *v1.Namespace:
			intf := client.CoreV1().Namespaces()
			existing, err := intf.Get(o.Name, metav1.GetOptions{})
			if err == nil && existing != nil {
				_, err2 = intf.Update(o)
			} else {
				_, err2 = intf.Create(o)
			}
		case *appsv1.Deployment:
			intf := client.AppsV1().Deployments(o.ObjectMeta.Namespace)
			existing, err := intf.Get(o.Name, metav1.GetOptions{})
			if err == nil && existing != nil {
				_, err2 = intf.Update(o)
			} else {
				_, err2 = intf.Create(o)
			}
		case *appsv1.StatefulSet:
			intf := client.AppsV1().StatefulSets(o.ObjectMeta.Namespace)
			existing, err := intf.Get(o.Name, metav1.GetOptions{})
			if err == nil && existing != nil {
				_, err2 = intf.Update(o)
			} else {
				_, err2 = intf.Create(o)
			}
		case *appsv1.DaemonSet:
			intf := client.AppsV1().DaemonSets(o.ObjectMeta.Namespace)
			existing, err := intf.Get(o.Name, metav1.GetOptions{})
			if err == nil && existing != nil {
				_, err2 = intf.Update(o)
			} else {
				_, err2 = intf.Create(o)
			}
		case *v1.ConfigMap:
			intf := client.CoreV1().ConfigMaps(o.ObjectMeta.Namespace)
			existing, err := intf.Get(o.Name, metav1.GetOptions{})
			if err == nil && existing != nil {
				_, err2 = intf.Update(o)
			} else {
				_, err2 = intf.Create(o)
			}
		case *rbacv1.Role:
			intf := client.RbacV1().Roles(o.ObjectMeta.Namespace)
			existing, err := intf.Get(o.Name, metav1.GetOptions{})
			if err == nil && existing != nil {
				_, err2 = intf.Update(o)
			} else {
				_, err2 = intf.Create(o)
			}
		case *rbacv1.ClusterRole:
			intf := client.RbacV1().ClusterRoles()
			existing, err := intf.Get(o.Name, metav1.GetOptions{})
			if err == nil && existing != nil {
				_, err2 = intf.Update(o)
			} else {
				_, err2 = intf.Create(o)
			}
		case *v1.ServiceAccount:
			intf := client.CoreV1().ServiceAccounts(o.ObjectMeta.Namespace)
			existing, err := intf.Get(o.Name, metav1.GetOptions{})
			if err == nil && existing != nil {
				_, err2 = intf.Update(o)
			} else {
				_, err2 = intf.Create(o)
			}
		case *rbacv1.RoleBinding:
			intf := client.RbacV1().RoleBindings(o.ObjectMeta.Namespace)
			existing, err := intf.Get(o.Name, metav1.GetOptions{})
			if err == nil && existing != nil {
				_, err2 = intf.Update(o)
			} else {
				_, err2 = intf.Create(o)
			}
		case *rbacv1.ClusterRoleBinding:
			intf := client.RbacV1().ClusterRoleBindings()
			existing, err := intf.Get(o.Name, metav1.GetOptions{})
			if err == nil && existing != nil {
				_, err2 = intf.Update(o)
			} else {
				_, err2 = intf.Create(o)
			}
		case *policyv1.PodSecurityPolicy:
			intf := client.PolicyV1beta1().PodSecurityPolicies()
			existing, err := intf.Get(o.Name, metav1.GetOptions{})
			if err == nil && existing != nil {
				_, err2 = intf.Update(o)
			} else {
				_, err2 = intf.Create(o)
			}
		default:
			err2 = fmt.Errorf("unknown type: %v", o)
		}

		if err2 != nil {
			return fmt.Errorf("error applying document %d: %v", i, err2)
		}
	}
	klog.V(2).Info("all manifests applied")
	return nil
}

func parseK8sYaml(fileR []byte) ([]runtime.Object, error) {

	acceptedK8sTypes := regexp.MustCompile(`(Role|ClusterRole|DaemonSet|RoleBinding|ClusterRoleBinding|ServiceAccount|Deployment|StatefulSet|Namespace|PodSecurityPolicy|ConfigMap)`)
	sepYamlFiles := bytes.Split(fileR, []byte("---"))
	retVal := make([]runtime.Object, 0, len(sepYamlFiles))
	for i, f := range sepYamlFiles {
		if len(f) == 0 || (len(f) == 1 && f[0] == 0x0a) {
			// ignore empty cases
			continue
		}

		decode := scheme.Codecs.UniversalDeserializer().Decode
		obj, groupVersionKind, err := decode(f, nil, nil)

		if err != nil {
			return nil, fmt.Errorf("error while decoding YAML object %d: %v", i, err)
		}

		if !acceptedK8sTypes.MatchString(groupVersionKind.Kind) {
			return nil, fmt.Errorf("manifest %d contained unsupport Kubernetes object type: %s", i, groupVersionKind.Kind)
		}
		retVal = append(retVal, obj)

	}
	return retVal, nil
}
