package vm

// Exported for tests in the vm_test package: both are unexported implementation details whose
// behaviour is worth asserting directly, because getting either wrong is silent.
var (
	SecretFileForTest     = secretFile
	ClassifyDockerForTest = classifyDocker
)
var ClassifyAppleForTest = classifyApple

// DecodeAppleForTest parses `container inspect` output without needing the runtime.
func DecodeAppleForTest(out string) ([]Machine, error) { return Apple{Bin: "container"}.decode(out) }

// WithSELinuxLabelForTest exposes the mount relabelling to the external tests.
var WithSELinuxLabelForTest = withSELinuxLabel
