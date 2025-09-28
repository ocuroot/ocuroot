package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/ocuroot/ocuroot"
	"github.com/ocuroot/ocuroot/about"
	"github.com/ocuroot/ocuroot/client"
	"github.com/ocuroot/ocuroot/client/local"
	"github.com/ocuroot/ocuroot/client/release"
	"github.com/ocuroot/ocuroot/client/work"
	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/sdk"
	"github.com/ocuroot/ocuroot/sdk/starlarkerrors"
	"github.com/spf13/cobra"
)

var MCPCommand = &cobra.Command{
	Use:   "mcp",
	Short: "Run an MCP (Model Context Protocol) server",
	Long:  `Run an MCP (Model Context Protocol) server to provide context to a model for a release.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Create a new MCP server
		s := server.NewMCPServer(
			"Ocuroot",
			about.Version,
			server.WithToolCapabilities(false),
		)

		// Add tool
		previewTool := mcp.NewTool("preview",
			mcp.WithDescription("Validate and preview the configuration of a package file"),
			mcp.WithString("package_file",
				mcp.Required(),
				mcp.Description("Full path to the package.ocu.star file to validate and preview"),
			),
		)

		// Add tool handler
		s.AddTool(previewTool, previewHandler)

		sdkTool := mcp.NewTool("sdk",
			mcp.WithDescription("Get stubs for the Ocuroot SDK, which provides guidance on how the SDK can be used to write package.ocu.star files."),
			mcp.WithString("version",
				mcp.Required(),
				mcp.Description("Version of the Ocuroot SDK to use. Use 'latest' for the latest version."),
			),
		)

		s.AddTool(sdkTool, sdkHandler)

		exampleListTool := mcp.NewTool("example_list",
			mcp.WithDescription("List available examples"),
		)

		s.AddTool(exampleListTool, exampleListHandler)

		exampleGetTool := mcp.NewTool("example_get",
			mcp.WithDescription("Get an example package file"),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Name of the example to get"),
			),
		)

		s.AddTool(exampleGetTool, exampleGetHandler)

		// Start the stdio server
		if err := server.ServeStdio(s); err != nil {
			fmt.Printf("Server error: %v\n", err)
		}
	},
}

func previewHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pkgFile, ok := request.GetArguments()["package_file"].(string)
	if !ok {
		return nil, errors.New("package_file must be a string")
	}

	repoRootPath, err := client.FindRepoRoot(path.Dir(pkgFile))
	if err != nil {
		return nil, fmt.Errorf("failed to find repo root: %w", err)
	}

	relativePackagePath, err := filepath.Rel(repoRootPath, pkgFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get relative package path: %w", err)
	}

	ref := refs.Ref{
		Repo:     "preview.git",
		Filename: relativePackagePath,
	}
	if strings.HasSuffix(pkgFile, "package.ocu.star") {
		ref.Filename = path.Dir(relativePackagePath)
	}

	fmt.Println("repo: ", repoRootPath)
	fmt.Println("ref: ", ref)

	w, err := work.NewWorker(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to create worker: %w", err)
	}
	defer w.Cleanup()

	backend, _ := local.NewBackend(ref)
	backend.Environments = &release.EnvironmentBackend{
		State: w.Tracker.State,
	}

	config, err := local.ExecutePackage(ctx, repoRootPath, ref, backend)
	if err != nil {
		return nil, fmt.Errorf("failed to load config at ref %s in repo %s: %v", ref, repoRootPath, starlarkerrors.Render(err))
	}

	packageJSON, err := json.Marshal(config.Package)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal package: %w", err)
	}

	return mcp.NewToolResultText(string(packageJSON)), nil
}

func sdkHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	version, ok := request.GetArguments()["version"].(string)
	if !ok {
		return nil, errors.New("version must be a string")
	}

	versions := sdk.AvailableVersions()
	if len(versions) == 0 {
		return nil, errors.New("no SDK versions found")
	}
	if version == "latest" {
		version = versions[len(versions)-1]
	}

	if !slices.Contains(versions, version) {
		return nil, fmt.Errorf("version %s not found", version)
	}

	stubs := sdk.GetVersionStubs(version)
	var sb strings.Builder

	for _, filename := range slices.Sorted(maps.Keys(stubs)) {
		contents := stubs[filename]
		sb.WriteString("#### " + filename + " ####\n")
		sb.WriteString(contents)
		sb.WriteString("\n####\n\n")
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func exampleListHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	exampleNames, err := fs.ReadDir(ocuroot.Examples, "examples")
	if err != nil {
		return nil, fmt.Errorf("failed to read example directory: %w", err)
	}

	var sb strings.Builder
	for _, info := range exampleNames {
		sb.WriteString(info.Name() + "\n")
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func exampleGetHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	exampleName, ok := request.GetArguments()["name"].(string)
	if !ok {
		return nil, errors.New("name must be a string")
	}

	exampleDirPath := fmt.Sprintf("examples/%s", exampleName)
	exampleEntries, err := fs.ReadDir(ocuroot.Examples, exampleDirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read example %s: %w", exampleName, err)
	}

	var exampleStarBytes []byte
	for _, entry := range exampleEntries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".star" {
			continue
		}
		exampleStarBytes, err = fs.ReadFile(ocuroot.Examples, fmt.Sprintf("%s/%s", exampleDirPath, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read example %s's .star file: %w", exampleName, err)
		}
		break
	}

	if exampleStarBytes == nil {
		return nil, fmt.Errorf("no .star file found in example %s", exampleName)
	}

	return mcp.NewToolResultText(string(exampleStarBytes)), nil
}

func init() {
	RootCmd.AddCommand(MCPCommand)
}
