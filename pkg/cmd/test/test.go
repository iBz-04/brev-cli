package test

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/brevdev/brev-cli/pkg/autostartconf"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

var (
	startLong    = "[internal] test"
	startExample = "[internal] test"
)

type TestStore interface {
	completions.CompletionStore
	ResetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetAllWorkspaces(options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetWorkspace(id string) (*entity.Workspace, error)
	GetWorkspaceMetaData(workspaceID string) (*entity.WorkspaceMetaData, error)
	CopyBin(targetBin string) error
	GetSetupScriptContentsByURL(url string) (string, error)
	UpdateUser(userID string, updatedUser *entity.UpdateUser) (*entity.User, error)
}

type ServiceMeshStore interface {
	autostartconf.AutoStartStore
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
}

func TypeItToMe(s string) {
	sleepSpeed := 21

	sRunes := []rune(s)
	for i := 0; i < len(sRunes); i++ {
		time.Sleep(time.Duration(sleepSpeed) * time.Millisecond)
		fmt.Printf("%c", sRunes[i])
	}
}

func NewCmdTest(_ *terminal.Terminal, _ TestStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"devonly": ""},
		Use:                   "test",
		DisableFlagsInUseLine: true,
		Short:                 "[internal] Test random stuff.",
		Long:                  startLong,
		Example:               startExample,
		// Args:                  cmderrors.TransformToValidationError(cobra.MinimumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			t := terminal.New()
			// s := t.NewSpinner()

			// TypeItToMe("PyEnvGPT is starting 🤙\n")
			TypeItToMe("\nPyEnvGPT is starting 🤙\n")
			fmt.Println("")

			// %%%%%%%%% Detect Operating System %%%%%%%%%
			str := t.Yellow("Detecting Operating System... ") + t.Green(getOperatingSystem()) + "\n"
			TypeItToMe(str)
			fmt.Println("")

			// %%%%%%%%% Detect Python Versions -- both of them %%%%%%%%%
			str = t.Yellow("Detecting Python2 Version... ") + t.Green(getPythonVersion()) + "\n"
			TypeItToMe(str)
			str = t.Yellow("Detecting Python3 Version... ") + t.Green(getPython3Version()) + "\n"
			TypeItToMe(str)
			fmt.Println("")

			// %%%%%%%%% Detect Virtual Environment %%%%%%%%%
			str = t.Yellow("Detecting Virtual Environment... ") + t.Green(getEnvironmentDetails()) + "\n"
			TypeItToMe(str)
			fmt.Println("")

			var pythonVersion string
			prompt := &survey.Select{
				Message: "Which Python version would you like to install?:",
				Options: []string{"Python 2.7", "Python 3.6", "Python 3.7", "Python 3.8", "Python 3.9", "Python 3.10"},
			}
			survey.AskOne(prompt, &pythonVersion)

			var installationMethod string
			prompt = &survey.Select{
				Message: "How would you like to install Python?:",
				Options: []string{"OS install", "virtualenv", "conda"},
			}
			survey.AskOne(prompt, &installationMethod)

			fmt.Printf("You chose to install %s using %s.\n", pythonVersion, installationMethod)
			// Based on the responses, you can now run the appropriate installation functions.

			if installationMethod == "virtualenv" {
				err := installPythonWithVenv(pythonVersion)
				if err != nil {
					return err
				}
			}
			if installationMethod == "source" {
				err := installPythonFromSource(pythonVersion)
				if err != nil {
					return err
				}
			}

			return nil
		},
	}

	return cmd
}

func runCommandWithOutput(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func installPythonWithVenv(version string) error {
	t := terminal.New()

	version = strings.Replace(version, "Python ", "", -1) // Replace "Python " with ""

	fmt.Printf("Checking for Python %s...\n", version)
	checkCmd := exec.Command("python"+version, "--version")

	// Try running python with the desired version
	if err := checkCmd.Run(); err != nil {
		fmt.Println(t.Red("Python %s is not installed.\n", version))
		fmt.Println(t.Red("Please install the desired version of Python and try again.\n"))
		return err
	}

	t.Printf("Python %s is installed. Creating a new virtual environment...\n", version)

	// Create a new virtual environment using Python's built-in venv module
	venvCmd := exec.Command("python"+version, "-m", "venv", "myenv")
	if err := runCommandWithOutput(venvCmd); err != nil {
		t.Printf(t.Red("Failed to create a new virtual environment: %s\n", err))
		return err
	}

	t.Printf("Created a new virtual environment at myenv.\n")
	t.Printf("You can activate it using the following command:\n")

	// Provide instructions for activating the virtual environment, which depends on the user's shell
	switch shell := runtime.GOOS; shell {
	case "windows":
		t.Printf("myenv\\Scripts\\activate\n")
	default:
		t.Printf("source myenv/bin/activate\n")
	}

	return nil
}

func getOperatingSystem() string {
	return runtime.GOOS
}

func getPythonVersion() string {
	cmd := exec.Command("python", "--version")
	output, err := cmd.CombinedOutput()
	pythonPath, _ := exec.LookPath("python")
	if err != nil {
		return "Python 2 not detected"
	}
	return strings.TrimSpace(string(output)) + "\n\tPath: " + pythonPath
}

func getPython3Version() string {
	cmd := exec.Command("python3", "--version")
	output, err := cmd.CombinedOutput()
	pythonPath, _ := exec.LookPath("python3")
	if err != nil {
		return "Python 3 not detected"
	}
	return strings.TrimSpace(string(output)) + "\n\tPath: " + pythonPath
}

func getEnvironmentDetails() string {
	condaCmd := exec.Command("bash", "-c", "source activate; conda env list")
	condaOutput, err := condaCmd.CombinedOutput()
	if err == nil && strings.Contains(string(condaOutput), "*") {
		return "Conda is being used"
	}

	venv := exec.Command("bash", "-c", "if [ -z \"$VIRTUAL_ENV\" ]; then echo 'No'; else echo 'Yes'; fi")
	venvOutput, err := venv.CombinedOutput()
	if err == nil && strings.Contains(string(venvOutput), "Yes") {
		return "venv is being used"
	}

	return "None detected"
}

func installPythonFromSource(version string) error {
	t := terminal.New()
	t.Printf("Installing Python %s from source...\n", version)

	pythonURL := fmt.Sprintf("https://www.python.org/ftp/python/%s/Python-%s.tgz", version, version)
	downloadCmd := exec.Command("curl", "-O", pythonURL)
	if err := runCommandWithOutput(downloadCmd); err != nil {
		t.Printf(t.Red("Failed to download Python source code: %s\n", err))
		return err
	}

	extractCmd := exec.Command("tar", "-xvzf", fmt.Sprintf("Python-%s.tgz", version))
	if err := runCommandWithOutput(extractCmd); err != nil {
		t.Printf(t.Red("Failed to extract Python source code: %s\n", err))
		return err
	}

	configureCmd := exec.Command("./configure", "--enable-optimizations", "--with-ensurepip=install")
	configureCmd.Dir = fmt.Sprintf("Python-%s", version)
	if err := runCommandWithOutput(configureCmd); err != nil {
		t.Printf(t.Red("Failed to configure Python: %s\n", err))
		return err
	}

	buildCmd := exec.Command("make", "-j8")
	buildCmd.Dir = fmt.Sprintf("Python-%s", version)
	if err := runCommandWithOutput(buildCmd); err != nil {
		t.Printf(t.Red("Failed to build Python: %s\n", err))
		return err
	}

	installCmd := exec.Command("sudo", "make", "altinstall")
	installCmd.Dir = fmt.Sprintf("Python-%s", version)
	if err := runCommandWithOutput(installCmd); err != nil {
		t.Printf(t.Red("Failed to install Python: %s\n", err))
		return err
	}

	t.Printf("Python %s has been successfully installed.\n", version)

	return nil
}
