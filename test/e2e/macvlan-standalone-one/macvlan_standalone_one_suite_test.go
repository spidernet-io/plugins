package macvlan_standalone_one_test

import (
	"context"
	"fmt"
	multus_v1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/cni-plugins/pkg/schema"
	"github.com/spidernet-io/cni-plugins/test/e2e/common"
	e2e "github.com/spidernet-io/e2eframework/framework"
	"github.com/spidernet-io/e2eframework/tools"
	spiderdoctorV1 "github.com/spidernet-io/spiderdoctor/pkg/k8s/apis/spiderdoctor.spidernet.io/v1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"testing"
	"time"
)

func TestMacvlanStandaloneOne(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MacvlanStandaloneOne Suite")
}

var frame *e2e.Framework
var name string
var spiderDoctorAgent *appsv1.DaemonSet
var annotations = make(map[string]string)
var successRate = float64(1)
var delayMs = int64(15000)
var (
	task        *spiderdoctorV1.Nethttp
	plan        *spiderdoctorV1.SchedulePlan
	target      *spiderdoctorV1.NethttpTarget
	targetAgent *spiderdoctorV1.TargetAgentSepc
	request     *spiderdoctorV1.NethttpRequest
	condition   *spiderdoctorV1.NetSuccessCondition
	run         = true
)

var _ = BeforeSuite(func() {
	defer GinkgoRecover()
	var e error
	task = new(spiderdoctorV1.Nethttp)
	plan = new(spiderdoctorV1.SchedulePlan)
	target = new(spiderdoctorV1.NethttpTarget)
	targetAgent = new(spiderdoctorV1.TargetAgentSepc)
	request = new(spiderdoctorV1.NethttpRequest)
	condition = new(spiderdoctorV1.NetSuccessCondition)

	frame, e = e2e.NewFramework(GinkgoT(), []func(*runtime.Scheme) error{multus_v1.AddToScheme, schema.SpiderPoolAddToScheme, spiderdoctorV1.AddToScheme})
	Expect(e).NotTo(HaveOccurred())

	name = "one-macvlan-standalone-" + tools.RandomName()

	// get macvlan-standalone multus crd instance by name
	multusInstance, err := frame.GetMultusInstance(common.MacvlanStandaloneVlan0Name, common.MultusNs)
	Expect(err).NotTo(HaveOccurred())
	Expect(multusInstance).NotTo(BeNil())

	annotations[common.MultusDefaultAnnotationKey] = fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanStandaloneVlan0Name)

	GinkgoWriter.Printf("update spiderdoctoragent annotation: %v/%v annotation: %v \n", common.SpiderDoctorAgentNs, common.SpiderDoctorAgentDSName, annotations)
	spiderDoctorAgent, err = frame.GetDaemonSet(common.SpiderDoctorAgentDSName, common.SpiderDoctorAgentNs)
	Expect(err).NotTo(HaveOccurred())
	Expect(spiderDoctorAgent).NotTo(BeNil())

	spiderDoctorAgent.Spec.Template.Annotations = annotations
	err = frame.UpdateResource(spiderDoctorAgent)
	Expect(err).NotTo(HaveOccurred())

	ctx, cancel := context.WithTimeout(context.Background(), 2*common.CtxTimeout)
	defer cancel()
	nodeList, err := frame.GetNodeList()
	Expect(err).NotTo(HaveOccurred())
	err = frame.WaitPodListRunning(spiderDoctorAgent.Spec.Selector.MatchLabels, len(nodeList.Items), ctx)
	Expect(err).NotTo(HaveOccurred())

	time.Sleep(30 * time.Second)
})

var _ = AfterSuite(func() {
	err := frame.DeleteResource(task)
	Expect(err).NotTo(HaveOccurred(), "failed to delete spiderdoctor nethttp %v", name)
})
