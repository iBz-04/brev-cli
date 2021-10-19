package portforward

import (
	"os"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/cmd/portforward"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func RunPortForwardFromCommand(namespace string, podName string, ports []string, keyFile string, certFile string) {
	args := append([]string{podName}, ports...)

	streams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}

	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	kubeConfigFlags.Namespace = &namespace
	kubeConfigFlags.KeyFile = &keyFile
	kubeConfigFlags.CertFile = &certFile
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)

	tf := cmdutil.NewFactory(matchVersionKubeConfigFlags)
	cmd := portforward.NewCmdPortForward(tf, streams)
	cmd.Run(cmd, args)
}
