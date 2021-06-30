/*
Based on https://github.com/openshift/cluster-machine-approver/tree/master/pkg/controller
Cluster machine approver approves CSR's from machines, hence we cannot use the code from
the package for approving CSR's from BYOH instances which may not have reference to a
machine object
*/

package csr

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"net"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	certificates "k8s.io/api/certificates/v1"
	core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeTypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/windows-machine-config-operator/pkg/instances"
	"github.com/openshift/windows-machine-config-operator/pkg/secrets"
	"github.com/openshift/windows-machine-config-operator/pkg/signer"
	"github.com/openshift/windows-machine-config-operator/pkg/windows"
)

//+kubebuilder:rbac:groups="certificates.k8s.io",resources=certificatesigningrequests,verbs=update/approval
//+kubebuilder:rbac:groups="certificates.k8s.io",resources=certificatesigningrequests,verbs=get;list;watch
//+kubebuilder:rbac:groups="certificates.k8s.io",resources=signers,verbs=Approve,resourceNames=kubernetes.io/kube-apiserver-client-kubelet;kubernetes.io/kubelet-serving

// kubeletClientUsages contains the permitted key usages from a kube-apiserver-client-kubelet signer
var kubeletClientUsages = []certificates.KeyUsage{
	certificates.UsageKeyEncipherment,
	certificates.UsageDigitalSignature,
	certificates.UsageClientAuth,
}

// kubeletServerUsages contains the permitted key usages from a kubelet-serving signer
var kubeletServerUsages = []certificates.KeyUsage{
	certificates.UsageKeyEncipherment,
	certificates.UsageDigitalSignature,
	certificates.UsageServerAuth,
}

const (
	nodeGroup          = "system:nodes"
	nodeUserName       = "system:node"
	NodeUserNamePrefix = nodeUserName + ":"
	systemPrefix       = "system:authenticated"
)

// Approver holds the information required to approve a node CSR
type Approver struct {
	// client is the cache client
	client client.Client
	// csr holds the pointer to the CSR request
	csr      *certificates.CertificateSigningRequest
	log      logr.Logger
	recorder record.EventRecorder
	// namespace is the namespace in which CSR's are present
	namespace string
}

// NewApprover returns a pointer to the Approver
func NewApprover(client client.Client, csr *certificates.CertificateSigningRequest,
	log logr.Logger, recorder record.EventRecorder, watchNamespace string) (*Approver, error) {
	if client == nil || csr == nil {
		return nil, errors.New(" kubernetes client, CSR should not be nil")
	}
	return &Approver{client,
		csr,
		log,
		recorder,
		watchNamespace}, nil
}

// Approve approves a CSR by updating it's status conditions to true if it is a valid CSR
func (a *Approver) Approve(clientSet *kubernetes.Clientset) error {
	if clientSet == nil {
		return errors.New("Kubernetes clientSet should not be nil")
	}

	if validated, err := a.validateCSRContents(); !validated {
		a.log.Info("CSR contents are invalid for approval by WMCO CSR Approver", "CSR", a.csr.Name)
		return errors.Wrapf(err, "could not validate CSR %s contents for approval", a.csr.Name)
	}

	a.csr.Status.Conditions = append(a.csr.Status.Conditions, certificates.CertificateSigningRequestCondition{
		Type:           certificates.CertificateApproved,
		Status:         "True",
		Message:        "Approved by WMCO",
		LastUpdateTime: meta.Now(),
		Reason:         "WMCOApproved",
	})
	if _, err := clientSet.CertificatesV1().CertificateSigningRequests().UpdateApproval(context.Background(),
		a.csr.Name, a.csr, meta.UpdateOptions{}); err != nil {
		return errors.Wrapf(err, "could not update conditions for CSR approval %s", a.csr.Name)
	}
	a.log.Info("CSR approved", "CSR", a.csr.Name)
	return nil
}

// validateCSRContents returns true if the CSR request contents are valid
func (a *Approver) validateCSRContents() (bool, error) {
	configMap := &core.ConfigMap{}
	err := a.client.Get(context.TODO(), kubeTypes.NamespacedName{Namespace: a.namespace,
		Name: "windows-instances"}, configMap)
	if k8sapierrors.IsNotFound(err) {
		// configMap hasn't been created yet, no action required
		return false, nil
	}
	if err != nil && !k8sapierrors.IsNotFound(err) {
		return false, errors.Wrapf(err, "could not retrieve Windows instance configMap %s", "windows-instances")
	}
	parsedCSR, err := ParseCSR(a.csr.Spec.Request)
	if err != nil {
		return false, errors.Wrapf(err, "error parsing CSR %s", a.csr.Name)
	}

	nodeName := strings.TrimPrefix(parsedCSR.Subject.CommonName, NodeUserNamePrefix)
	if nodeName == "" {
		return false, errors.Errorf("CSR subject name does not contain the required node user prefix %s", a.csr.Name)
	}

	// lookup the node name against the instance configMap/host names
	if valid, err := a.validateNodeName(nodeName, configMap); !valid {
		// CSR is not from a BYOH Windows instance, don't deny it or return error
		// since it might be from a linux node.
		return false, errors.Wrapf(err, "error validating node name %s for CSR %s", nodeName, a.csr.Name)
	}
	// Kubelet on a node needs two certificates for its normal operation:
	// Client certificate for securely communicating with the Kubernetes API server
	// Server certificate for use by Kubernetes API server to talk back to kubelet
	// Both types are validated based on their contents
	if a.isNodeClientCert(parsedCSR) {
		// Node client bootstrapper CSR is received before the instance becomes a node
		// hence we should not proceed if a corresponding node already exists
		node := &core.Node{}
		err := a.client.Get(context.TODO(), kubeTypes.NamespacedName{Namespace: a.namespace,
			Name: nodeName}, node)
		if err != nil && !apierrors.IsNotFound(err) {
			return false, errors.Wrapf(err, "unable to get node %s", nodeName)
		} else if err == nil {
			return false, errors.Wrapf(err, "%s node already exists, cannot validate CSR", nodeName)
		}
	} else {
		if validated, err := a.validateKubeletServingCSR(parsedCSR); !validated {
			return false, errors.Wrapf(err, "unable to validate kubelet serving CSR %s", a.csr.Name)
		}
	}
	return true, nil
}

// validateNodeName returns true if the node name passed here matches
// either the actual host name of the VM'S or the reverse lookup of
// the instance addresses present in the configMap
func (a *Approver) validateNodeName(nodeName string, configMap *core.ConfigMap) (bool, error) {
	// Get the list of instances that are expected to be Nodes
	windowsInstances, err := instances.ParseInstances(configMap.Data)
	if err != nil {
		return false, errors.Wrapf(err, "unable to parse hosts from ConfigMap %s", configMap.Name)
	}
	// check if the node name matches the lookup of instance addresses
	for _, instance := range windowsInstances {
		found, err := mapNodeToInstance(instance.Address, nodeName)
		if err != nil {
			return false, errors.Wrapf(err, "unable to map node name to instance with address %s",
				instance.Address)
		}
		if found {
			return true, nil
		}
	}
	// find the host name for the instance and check if it matches node name
	for _, instance := range windowsInstances {
		hostName, err := a.findInstanceHostName(instance)
		if err != nil {
			return false, errors.Wrapf(err, "unable to find host name for instance with address %s",
				instance.Address)
		}
		// validate host name complies with DNS RFC1123 naming convention
		// ref: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names
		if errs := validation.IsDNS1123Subdomain(hostName); len(errs) > 0 {
			a.recorder.Eventf(a.csr, core.EventTypeWarning, "hostNameValidationFailed",
				"host name %s does not comply with naming rules defined in RFC1123: "+
					"Requirements for internet hosts", hostName)
			return false, errors.Errorf("host name %s does not comply with naming rules defined in RFC1123: "+
				"Requirements for internet hosts", hostName)
		}
		// check if the instance host name matches node name
		if strings.Contains(hostName, nodeName) {
			return true, nil
		}
	}
	return false, nil
}

// mapNodeToInstance returns true if the DNS address in CSR subject matches with the instance address
// If the address passed is an IP address, we do a reverse lookup to find the DNS address
func mapNodeToInstance(instanceAdd string, nodeName string) (bool, error) {
	// reverse lookup the instance if the address is an IP address
	if parseAddr := net.ParseIP(instanceAdd); parseAddr != nil {
		dnsAddresses, err := net.LookupAddr(instanceAdd)
		if err != nil {
			return false, errors.Wrapf(err, "failed to lookup DNS for IP %s", instanceAdd)
		}
		for _, dns := range dnsAddresses {
			if strings.Contains(dns, nodeName) {
				return true, nil
			}
		}
	} else { // direct match if it is a DNS address
		if strings.Contains(instanceAdd, nodeName) {
			return true, nil
		}
	}
	return false, nil
}

// findInstanceHostName returns the instance's actual host name by running the 'hostname' command on the VM
func (a *Approver) findInstanceHostName(instance *instances.InstanceInfo) (string, error) {
	var err error
	// Create a new signer using the private key secret
	instanceSigner, err := signer.Create(kubeTypes.NamespacedName{Namespace: a.namespace,
		Name: secrets.PrivateKeySecret}, a.client)
	if err != nil {
		return "", errors.Wrapf(err, "unable to create signer from private key secret")
	}
	win, err := windows.New("", "", instance, instanceSigner)
	if err != nil {
		return "", errors.Wrap(err, "error instantiating Windows instance")
	}
	// get the VM host name  by running hostname command on remote VM
	hostName, err := win.Run("hostname", false)
	if err != nil {
		return "", errors.Wrapf(err, "error getting the host name, with stdout %s", hostName)
	}
	return hostName, nil
}

// validateKubeletServingCSR validates a kubelet serving CSR for its contents
func (a *Approver) validateKubeletServingCSR(parsedCsr *x509.CertificateRequest) (bool, error) {
	if a.csr == nil || parsedCsr == nil {
		return false, errors.New("CSR or request should not be nil")
	}

	// Check groups, we need at least: system:nodes, system:authenticated
	if len(a.csr.Spec.Groups) < 2 {
		return false, errors.Errorf("CSR %s contains invalid number of groups: %d", a.csr.Name,
			len(a.csr.Spec.Groups))
	}
	groups := sets.NewString(a.csr.Spec.Groups...)
	if !groups.HasAll(nodeGroup, systemPrefix) {
		return false, errors.Errorf("CSR %s does not contain required groups", a.csr.Name)
	}

	// Check usages include: digital signature, key encipherment and server auth
	if !a.hasUsages(kubeletServerUsages) {
		return false, errors.Errorf("CSR %s does not contain required usages", a.csr.Name)
	}

	var hasOrg bool
	for i := range parsedCsr.Subject.Organization {
		if parsedCsr.Subject.Organization[i] == nodeGroup {
			hasOrg = true
			break
		}
	}
	if !hasOrg {
		return false, errors.Errorf("CSR %s does not contain required subject organization", a.csr.Name)
	}
	return true, nil
}

// isNodeClientCert returns true if the CSR is from a  kube-apiserver-client-kubelet signer
// reference: https://kubernetes.io/docs/reference/access-authn-authz/certificate-signing-requests/#kubernetes-signers
func (a *Approver) isNodeClientCert(x509cr *x509.CertificateRequest) bool {
	if !reflect.DeepEqual([]string{nodeGroup}, x509cr.Subject.Organization) {
		return false
	}
	if (len(x509cr.DNSNames) > 0) || (len(x509cr.EmailAddresses) > 0) || (len(x509cr.IPAddresses) > 0) {
		return false
	}
	// Check usages include: digital signature, key encipherment and client auth
	if !a.hasUsages(kubeletClientUsages) {
		return false
	}
	return true
}

// hasUsages verifies if the required usages exist in the CSR spec
func (a *Approver) hasUsages(usages []certificates.KeyUsage) bool {
	if len(usages) != len(a.csr.Spec.Usages) {
		return false
	}

	usageMap := map[certificates.KeyUsage]struct{}{}
	for _, u := range usages {
		usageMap[u] = struct{}{}
	}

	for _, u := range a.csr.Spec.Usages {
		if _, ok := usageMap[u]; !ok {
			return false
		}
	}

	return true
}

// ParseCSR extracts the CSR from the API object and decodes it.
func ParseCSR(csr []byte) (*x509.CertificateRequest, error) {
	if len(csr) == 0 {
		return nil, errors.New("CSR request spec should not be empty")
	}
	// extract PEM from request object
	block, _ := pem.Decode(csr)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, errors.New("PEM block type must be CERTIFICATE REQUEST")
	}
	return x509.ParseCertificateRequest(block.Bytes)
}
