package updatemodel

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/hashicorp/go-multierror"
	"github.com/samber/lo"
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/autostartconf"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/brevdev/parse/pkg/parse"
	"github.com/go-git/go-git/v5"
)

var (
	short   = "TODO"
	long    = "TODO"
	example = "TODO"
)

type updatemodelStore interface {
	ModifyWorkspace(workspaceID string, options *store.ModifyWorkspaceRequest) (*entity.Workspace, error)
	GetCurrentWorkspaceID() (string, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	WriteString(path, data string) error
	UserHomeDir() (string, error)
}

func NewCmdupdatemodel(t *terminal.Terminal, store updatemodelStore) *cobra.Command {
	var configure bool
	cmd := &cobra.Command{
		Use:                   "updatemodel",
		DisableFlagsInUseLine: true,
		Short:                 short,
		Long:                  long,
		Example:               example,
		RunE: updateModel{
			t:     t,
			Store: store,
			clone: git.PlainClone,
			open: func(path string) (repo, error) {
				r, err := git.PlainOpen(path)
				return r, breverrors.WrapAndTrace(err)
			},
			configure: configure,
		}.RunE,
	}

	cmd.Flags().BoolVarP(&configure, "configure", "c", false, "configure daemon")
	return cmd
}

type repo interface {
	Remotes() ([]*git.Remote, error)
}

type updateModel struct {
	t         *terminal.Terminal
	Store     updatemodelStore
	clone     func(path string, isBare bool, o *git.CloneOptions) (*git.Repository, error)
	open      func(path string) (repo, error)
	configure bool
}

func (u updateModel) RunE(_ *cobra.Command, _ []string) error {
	if u.configure {
		return breverrors.WrapAndTrace(
			DaemonConfigurer{
				Store: u.Store,
			}.Configure(),
		)
	}

	remotes, err := u.remotes()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	workspaceID, err := u.Store.GetCurrentWorkspaceID()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	workspace, err := u.Store.GetWorkspace(workspaceID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	urls := lo.Map(
		remotes,
		func(remote *git.Remote, _ int) string {
			return remote.Config().URLs[0]
		},
	)

	reposv1FromBE := workspace.ReposV1
	reposv1FromENV := makeReposFromRemotes(urls)

	rm := &repoMerger{
		acc:   reposv1FromBE,
		repos: []*entity.ReposV1{reposv1FromENV},
	}
	_, err = u.Store.ModifyWorkspace(
		workspaceID,
		&store.ModifyWorkspaceRequest{
			ReposV1: rm.MergeBE(),
		},
	)

	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	dir, err := u.Store.UserHomeDir()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	errors := lo.Map(
		rm.ReposToClone(),
		func(repo *entity.RepoV1, _ int) error {
			_, err := u.clone(dir, false, &git.CloneOptions{
				URL: repo.GitRepo.Repository,
			})
			return breverrors.WrapAndTrace(err)
		},
	)
	return breverrors.WrapAndTrace(
		lo.Reduce(
			errors,
			func(acc error, err error, _ int) error {
				if acc != nil && err != nil {
					return multierror.Append(acc, err)
				}
				if acc == nil && err != nil {
					return breverrors.WrapAndTrace(err)
				}
				return acc
			},
			nil,
		),
	)
}

type stringWriter interface {
	WriteString(path, data string) error
}

type DaemonConfigurer struct {
	Store stringWriter
}

func (dc DaemonConfigurer) Configure() error {
	// create systemd service file to run
	// brev updatemodel -d /home/ubuntu
	configFile := filepath.Join("/etc/systemd/system", "brev-updatemodel.service")
	err := dc.Store.WriteString(
		configFile,
		`[Unit]
Description=Brev Update Model
After=network.target

[Service]
Type=simple
User=ubuntu
ExecStart=/usr/bin/brev updatemodel -d /home/ubuntu
`)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if autostartconf.ShouldSymlink() {
		symlinkTarget := path.Join("/etc/systemd/system/default.target.wants/", "brev-updatemodel.service")
		err2 := os.Symlink(configFile, symlinkTarget)
		if err2 != nil {
			return breverrors.WrapAndTrace(err2)
		}
	}
	// create systemd timer to run every 5 seconds
	err = dc.Store.WriteString(
		"/etc/systemd/system/brev-updatemodel.timer",
		`[Unit]
Description=Brev Update Model Timer

[Timer]
OnBootSec=5
OnUnitActiveSec=5

[Install]
WantedBy=timers.target
`)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// enable timer
	err = autostartconf.ExecCommands(
		[][]string{
			{"systemctl", "enable", "brev-updatemodel.timer"},
			{"systemctl", "start", "brev-updatemodel.timer"},
			{"systemctl", "daemon-reload"},
		},
	)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func makeReposFromRemotes(remotes []string) *entity.ReposV1 {
	return lo.Reduce(
		remotes,
		func(acc *entity.ReposV1, remote string, _ int) *entity.ReposV1 {
			name := parse.GetRepoNameFromOrigin(remote)
			url := parse.GetSSHURLFromOrigin(remote)
			a := *acc
			a[entity.RepoName(name)] = entity.RepoV1{
				Type: entity.GitRepoType,
				GitRepo: entity.GitRepo{
					Repository: url,
				},
			}
			return &a
		},
		&entity.ReposV1{},
	)
}

type repoMerger struct {
	acc   *entity.ReposV1
	repos []*entity.ReposV1
}

func (r *repoMerger) MergeBE() *entity.ReposV1 {
	for _, repo := range r.repos {
		for k, v := range *repo {
			if _, ok := (*r.acc)[k]; ok {
				continue
			}
			_, valueInAcc := lo.Find(
				r.accValues(),
				func(repo *entity.RepoV1) bool {
					return repo.GitRepo.Repository == v.GitRepo.Repository
				},
			)
			if valueInAcc {
				continue
			}
			(*r.acc)[k] = v
		}
	}
	return r.acc
}

func (r *repoMerger) ReposToClone() []*entity.RepoV1 {
	// repos present in the BE but not in the ENV
	return lo.Filter(
		r.accValues(),
		func(accrepo *entity.RepoV1, _ int) bool {
			_, valueInENV := lo.Find(
				r.reposValues(),
				func(repo *entity.RepoV1) bool {
					return accrepo.GitRepo.Repository == repo.GitRepo.Repository
				},
			)
			return !valueInENV
		},
	)
}

func (r repoMerger) reposValues() []*entity.RepoV1 {
	values := []*entity.RepoV1{}
	for _, repo := range r.repos {
		for _, v := range *repo {
			// explicit memory aliasing in for loop.
			v := v
			values = append(values, &v)
		}
	}
	return values
}

func (r repoMerger) accValues() []*entity.RepoV1 {
	if r.acc == nil {
		return []*entity.RepoV1{}
	}
	values := []*entity.RepoV1{}
	for _, v := range *r.acc {
		values = append(values, &v)
	}
	return values
}

func (u updateModel) remotes() ([]*git.Remote, error) {
	remotes := []*git.Remote{}
	dir, err := u.Store.UserHomeDir()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	err = filepath.WalkDir(dir,
		func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			if !d.IsDir() {
				return nil
			}
			repo, err := git.PlainOpen(path)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			remotes, err := repo.Remotes()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			remotes = append(remotes, remotes...)
			return nil
		},
	)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return remotes, nil
}