package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/yaml"

	metering "github.com/kube-reporting/metering-operator/pkg/apis/metering/v1"
	"github.com/kube-reporting/metering-operator/test/testhelpers"
)

func testManualMeteringInstall(
	t *testing.T,
	testCaseName,
	namespacePrefix,
	meteringOperatorImageRepo,
	meteringOperatorImageTag,
	manifestFilename,
	catalogSourceName,
	catalogSourceNamespace,
	subscriptionChannel,
	testOutputPath string,
	expectInstallErrMsg []string,
	expectInstallErr bool,
	testInstallFunction InstallTestCase,
) {
	// create a directory used to store the @testCaseName container and resource logs
	testCaseOutputBaseDir := filepath.Join(testOutputPath, testCaseName)
	err := os.Mkdir(testCaseOutputBaseDir, 0777)
	assert.NoError(t, err, "creating the test case output directory should produce no error")

	testFuncNamespace := fmt.Sprintf("%s-%s", namespacePrefix, strings.ToLower(testCaseName))
	if len(testFuncNamespace) > kubeNamespaceCharLimit {
		require.Fail(t, "The length of the test function namespace exceeded the kube namespace limit of %d characters", kubeNamespaceCharLimit)
	}

	mc, err := DecodeMeteringConfigManifest(repoPath, testMeteringConfigManifestsPath, manifestFilename)
	require.NoError(t, err, "failed to successfully decode the YAML MeteringConfig manifest")

	mc := &metering.MeteringConfig{}
	err = yaml.NewYAMLOrJSONDecoder(file, 100).Decode(&mc)
	require.NoError(t, err, "failed to decode the yaml meteringconfig manifest")
	require.NotNil(t, mc, "the decoded meteringconfig object is nil")

	deployerCtx, err := df.NewDeployerCtx(
		testFuncNamespace,
		meteringOperatorImageRepo,
		meteringOperatorImageTag,
		reportingOperatorImageRepo,
		reportingOperatorImageTag,
		catalogSourceName,
		catalogSourceNamespace,
		subscriptionChannel,
		testCaseOutputBaseDir,
		testInstallFunction.ExtraEnvVars,
		mc.Spec,
	)
	require.NoError(t, err, "creating a new deployer context should produce no error")
	defer deployerCtx.LoggerOutFile.Close()

	rf, err := deployerCtx.Setup(deployerCtx.Deployer.InstallOLM, expectInstallErr)

	canSafelyRunTest := testhelpers.AssertCanSafelyRunReportingTests(t, err, expectInstallErr, expectInstallErrMsg)
	if canSafelyRunTest {
		t.Run(testInstallFunction.Name, func(t *testing.T) {
			testInstallFunction.TestFunc(t, rf)
		})

		deployerCtx.Logger.Infof("The %s test has finished running", testInstallFunction.Name)
	}

	err = deployerCtx.Teardown(deployerCtx.Deployer.UninstallOLM)
	assert.NoError(t, err, "capturing logs and uninstalling metering should produce no error")
}

func DecodeMeteringConfigManifest(basePath, manifestPath, manifestFilename string) (*metering.MeteringConfig, error) {
	manifestFullPath := filepath.Join(basePath, manifestPath, manifestFilename)
	file, err := os.Open(manifestFullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open the %s manifest file: %v", manifestFullPath, err)
	}

	mc := &metering.MeteringConfig{}
	err = yaml.NewYAMLOrJSONDecoder(file, 100).Decode(&mc)
	if err != nil {
		return nil, err
	}

	if mc == nil {
		return nil, fmt.Errorf("error: the decoded MeteringConfig object is nil")
	}

	return mc, nil
}