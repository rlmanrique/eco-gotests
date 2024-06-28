package ran_du_system_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-kni/eco-goinfra/pkg/namespace"
	"github.com/openshift-kni/eco-goinfra/pkg/reportxml"
	"github.com/openshift-kni/eco-gotests/tests/system-tests/internal/await"
	"github.com/openshift-kni/eco-gotests/tests/system-tests/internal/platform"
	"github.com/openshift-kni/eco-gotests/tests/system-tests/internal/shell"
	"github.com/openshift-kni/eco-gotests/tests/system-tests/internal/stability"
	. "github.com/openshift-kni/eco-gotests/tests/system-tests/ran-du/internal/randuinittools"
	"github.com/openshift-kni/eco-gotests/tests/system-tests/ran-du/internal/randuparams"
)

var _ = Describe(
	"StabilityWorkload",
	Ordered,
	ContinueOnFailure,
	Label("StabilityWorkload"), func() {
		var (
			clusterName string
		)
		BeforeAll(func() {
			By("Preparing workload")
			if namespace.NewBuilder(APIClient, RanDuTestConfig.TestWorkload.Namespace).Exists() {
				By("Deleting workload using shell method")
				_, err := shell.ExecuteCmd(RanDuTestConfig.TestWorkload.DeleteShellCmd)
				Expect(err).ToNot(HaveOccurred(), "Failed to delete workload")
			}

			if RanDuTestConfig.TestWorkload.CreateMethod == randuparams.TestWorkloadShellLaunchMethod {
				By("Launching workload using shell method")
				_, err := shell.ExecuteCmd(RanDuTestConfig.TestWorkload.CreateShellCmd)
				Expect(err).ToNot(HaveOccurred(), "Failed to launch workload")
			}

			By("Waiting for deployment replicas to become ready")
			_, err := await.WaitUntilAllDeploymentsReady(APIClient, RanDuTestConfig.TestWorkload.Namespace,
				randuparams.DefaultTimeout)
			Expect(err).ToNot(HaveOccurred(), "error while waiting for deployment to become ready")

			By("Waiting for statefulset replicas to become ready")
			_, err = await.WaitUntilAllStatefulSetsReady(APIClient, RanDuTestConfig.TestWorkload.Namespace,
				randuparams.DefaultTimeout)
			Expect(err).ToNot(HaveOccurred(), "error while waiting for statefulsets to become ready")

			By("Waiting for pods replicas to become ready")
			_, err = await.WaitUntilAllPodsReady(APIClient, RanDuTestConfig.TestWorkload.Namespace, randuparams.DefaultTimeout)
			Expect(err).ToNot(HaveOccurred(), "pod not ready: %s", err)

			By("Fetching Cluster name")
			clusterName, err = platform.GetOCPClusterName(APIClient)
			Expect(err).ToNot(HaveOccurred(), "Failed to get cluster name")

		})
		It("StabilityWorkload", reportxml.ID("42744"), Label("StabilityWorkload"), func() {

			outputDir := RanDuTestConfig.StabilityOutputPath
			policiesOutputFile := fmt.Sprintf("%s/stability_workload_policies.log", outputDir)
			ptpOutputFile := fmt.Sprintf("%s/stability_workload_ptp.log", outputDir)
			namespaces := []string{"openshift-etcd", "openshift-apiserver"}

			totalDuration := time.Duration(RanDuTestConfig.StabilityWorkloadDurMins) * time.Minute
			interval := time.Duration(RanDuTestConfig.StabilityWorkloadIntMins) * time.Minute
			startTime := time.Now()

			By(fmt.Sprintf("Collecting metrics during %d minutes", RanDuTestConfig.StabilityWorkloadDurMins))
			for time.Since(startTime) < totalDuration {

				if RanDuTestConfig.PtpEnabled {
					err := stability.SavePTPStatus(APIClient, ptpOutputFile, interval)
					if err != nil {
						fmt.Printf("Error, could not save PTP status")
					}
				}

				err := stability.SavePolicyStatus(APIClient, clusterName, policiesOutputFile)
				if err != nil {
					fmt.Printf("Error, could not save policies status")
				}
				for _, namespace := range namespaces {
					err = stability.SavePodsRestartsInNamespace(APIClient,
						namespace, fmt.Sprintf("%s/stability_workload_%s.log", outputDir, namespace))
					if err != nil {
						fmt.Printf("Error, could not save pod restarts")
					}
				}

				time.Sleep(interval)
			}

			// Final check of all values
			By("Check all results")
			var stabilityErrors []string

			// Verify policies
			By("Check Policy changes")
			_, err := stability.VerifyStabilityStatusChange(policiesOutputFile)
			if err != nil {
				stabilityErrors = append(stabilityErrors, err.Error())
			}

			// Verify podRestarts
			By("Check Pod restarts")
			for _, namespace := range namespaces {
				_, err := stability.VerifyStabilityStatusChange(fmt.Sprintf("%s/stability_workload_%s.log", outputDir, namespace))
				if err != nil {
					stabilityErrors = append(stabilityErrors, err.Error())
				}
			}

			// Verify PTP output
			By("Check PTP results")
			if RanDuTestConfig.PtpEnabled {
				_, err = stability.VerifyStabilityStatusChange(ptpOutputFile)
				if err != nil {
					stabilityErrors = append(stabilityErrors, err.Error())
				}
			}

			By("Check if there been any error")
			if len(stabilityErrors) > 0 {
				Expect(stabilityErrors).ToNot(HaveOccurred(), "One or more errors in stability tests: %s", stabilityErrors)
			}

		})
		AfterAll(func() {
			By("Cleaning up test workload resources")
			_, err := shell.ExecuteCmd(RanDuTestConfig.TestWorkload.DeleteShellCmd)
			Expect(err).ToNot(HaveOccurred(), "Failed to delete workload")
		})
	})
