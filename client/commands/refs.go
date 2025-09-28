package commands

import (
	"github.com/ocuroot/ocuroot/refs"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func GetRef(cmd *cobra.Command, args []string) (refs.Ref, error) {
	if len(args) > 0 {
		return refs.Parse(args[0])
	}

	out := refs.Ref{}
	if cmd != nil {
		out.Filename, _ = cmd.Flags().GetString("package")
		releaseName, _ := cmd.Flags().GetString("release")
		if releaseName != "" {
			out = out.SetRelease(releaseName)
		}
	}

	if out.Filename == "" {
		out.Filename = "."
	}

	return out, nil
}

func AddRefFlags(cmd *cobra.Command, persistent bool) {
	var flags *pflag.FlagSet
	if persistent {
		flags = cmd.PersistentFlags()
	} else {
		flags = cmd.Flags()
	}
	flags.String("package", ".", "Path to the working package in the current repository. Can also be specified via a full ref in the first parameter.")
	flags.String("release", "", "ID or tag of the release. Can also be specified via a full ref in the first parameter.")
}
