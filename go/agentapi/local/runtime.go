package local

type Runtime struct {
	AppName    string
	Dirs       AppDirs
	Files      *FileStore
	Data       *FileStore
	Cache      *FileStore
	Logs       *FileStore
	Temp       *FileStore
	Config     *ConfigStore
	Skills     *SkillStore
	Workdirs *WorkdirManager
}

type RuntimeOptions struct {
	AppName   string
	AppAuthor string
	BaseDir   string
	Dirs      map[string]string
	Env       map[string]string
	Platform  string
}

func NewRuntime(opts RuntimeOptions) (*Runtime, error) {
	appName := opts.AppName
	dirs, err := AppDirsFor(appName, AppDirsOptions{AppAuthor: opts.AppAuthor, BaseDir: opts.BaseDir, Dirs: opts.Dirs, Env: opts.Env, Platform: opts.Platform})
	if err != nil {
		return nil, err
	}
	data, err := NewFileStore(dirs.Data, "data")
	if err != nil {
		return nil, err
	}
	cache, _ := NewFileStore(dirs.Cache, "cache")
	logs, _ := NewFileStore(dirs.Logs, "logs")
	temp, _ := NewFileStore(dirs.Temp, "temp")
	configFiles, _ := NewFileStore(dirs.Config, "config")
	skillsFiles, _ := data.Child("skills", "skills")
	return &Runtime{
		AppName:    appName,
		Dirs:       dirs,
		Files:      data,
		Data:       data,
		Cache:      cache,
		Logs:       logs,
		Temp:       temp,
		Config:     &ConfigStore{Files: configFiles},
		Skills:     &SkillStore{Files: skillsFiles},
		Workdirs: &WorkdirManager{},
	}, nil
}

func (r *Runtime) Ensure() error {
	for _, store := range []*FileStore{r.Data, r.Cache, r.Logs, r.Temp, r.Config.Files} {
		if err := store.Ensure(); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) Workdir(root string, opts WorkdirOptions) (*Workdir, error) {
	return NewWorkdir(root, opts)
}
