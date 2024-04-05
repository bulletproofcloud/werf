package helm

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/werf/logboek"
	"github.com/werf/logboek/pkg/level"
	"github.com/werf/werf/cmd/werf/common"
	"github.com/werf/werf/pkg/build"
	"github.com/werf/werf/pkg/config/deploy_params"
	"github.com/werf/werf/pkg/deploy/helm/chart_extender/helpers"
	"github.com/werf/werf/pkg/git_repo"
	"github.com/werf/werf/pkg/git_repo/gitdata"
	"github.com/werf/werf/pkg/image"
	"github.com/werf/werf/pkg/ssh_agent"
	"github.com/werf/werf/pkg/storage/lrumeta"
	"github.com/werf/werf/pkg/storage/manager"
	"github.com/werf/werf/pkg/tmp_manager"
	"github.com/werf/werf/pkg/true_git"
	"github.com/werf/werf/pkg/util"
	"github.com/werf/werf/pkg/werf"
)

var getAutogeneratedValuedCmdData common.CmdData

func NewGetAutogeneratedValuesCmd(ctx context.Context) *cobra.Command {
	ctx = common.NewContextWithCmdData(ctx, &getAutogeneratedValuedCmdData)
	cmd := common.SetCommandContext(ctx, &cobra.Command{
		Use:                   "get-autogenerated-values [IMAGE_NAME...]",
		Short:                 "Get service values yaml generated by werf for helm chart during deploy",
		Long:                  common.GetLongCommandDescription(GetGetAutogeneratedValuesDocs().Long),
		DisableFlagsInUseLine: true,
		Annotations: map[string]string{
			common.CmdEnvAnno: common.EnvsDescription(common.WerfSecretKey),
			common.DocsLongMD: GetGetAutogeneratedValuesDocs().LongMD,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := common.ProcessLogOptions(&getAutogeneratedValuedCmdData); err != nil {
				common.PrintHelp(cmd)
				return err
			}

			return runGetServiceValues(ctx, common.GetImagesToProcess(args, *getAutogeneratedValuedCmdData.WithoutImages))
		},
	})

	getAutogeneratedValuedCmdData.SetupWithoutImages(cmd)

	common.SetupDir(&getAutogeneratedValuedCmdData, cmd)
	common.SetupGitWorkTree(&getAutogeneratedValuedCmdData, cmd)
	common.SetupConfigTemplatesDir(&getAutogeneratedValuedCmdData, cmd)
	common.SetupConfigPath(&getAutogeneratedValuedCmdData, cmd)
	common.SetupGiterminismConfigPath(&getAutogeneratedValuedCmdData, cmd)
	common.SetupEnvironment(&getAutogeneratedValuedCmdData, cmd)

	common.SetupGiterminismOptions(&getAutogeneratedValuedCmdData, cmd)

	common.SetupTmpDir(&getAutogeneratedValuedCmdData, cmd, common.SetupTmpDirOptions{})
	common.SetupHomeDir(&getAutogeneratedValuedCmdData, cmd, common.SetupHomeDirOptions{})
	common.SetupSSHKey(&getAutogeneratedValuedCmdData, cmd)

	common.SetupSecondaryStagesStorageOptions(&getAutogeneratedValuedCmdData, cmd)
	common.SetupCacheStagesStorageOptions(&getAutogeneratedValuedCmdData, cmd)
	common.SetupRepoOptions(&getAutogeneratedValuedCmdData, cmd, common.RepoDataOptions{})
	common.SetupFinalRepo(&getAutogeneratedValuedCmdData, cmd)

	common.SetupSynchronization(&getAutogeneratedValuedCmdData, cmd)
	common.SetupKubeConfig(&getAutogeneratedValuedCmdData, cmd)
	common.SetupKubeConfigBase64(&getAutogeneratedValuedCmdData, cmd)
	common.SetupKubeContext(&getAutogeneratedValuedCmdData, cmd)

	common.SetupUseCustomTag(&getAutogeneratedValuedCmdData, cmd)
	common.SetupVirtualMerge(&getAutogeneratedValuedCmdData, cmd)

	common.SetupNamespace(&getAutogeneratedValuedCmdData, cmd, true)

	common.SetupDockerConfig(&getAutogeneratedValuedCmdData, cmd, "Command needs granted permissions to read and pull images from the specified repo")
	common.SetupInsecureRegistry(&getAutogeneratedValuedCmdData, cmd)
	common.SetupSkipTlsVerifyRegistry(&getAutogeneratedValuedCmdData, cmd)

	common.SetupStubTags(&getAutogeneratedValuedCmdData, cmd)

	common.SetupLogOptions(&getAutogeneratedValuedCmdData, cmd)

	getAutogeneratedValuedCmdData.SetupPlatform(cmd)

	return cmd
}

func runGetServiceValues(ctx context.Context, imagesToProcess build.ImagesToProcess) error {
	logboek.SetAcceptedLevel(level.Error)

	if err := werf.Init(*getAutogeneratedValuedCmdData.TmpDir, *getAutogeneratedValuedCmdData.HomeDir); err != nil {
		return fmt.Errorf("initialization error: %w", err)
	}

	containerBackend, processCtx, err := common.InitProcessContainerBackend(ctx, &getAutogeneratedValuedCmdData)
	if err != nil {
		return err
	}
	ctx = processCtx

	gitDataManager, err := gitdata.GetHostGitDataManager(ctx)
	if err != nil {
		return fmt.Errorf("error getting host git data manager: %w", err)
	}

	if err := git_repo.Init(gitDataManager); err != nil {
		return err
	}

	if err := image.Init(); err != nil {
		return err
	}

	if err := lrumeta.Init(); err != nil {
		return err
	}

	if err := true_git.Init(ctx, true_git.Options{LiveGitOutput: *getAutogeneratedValuedCmdData.LogVerbose || *getAutogeneratedValuedCmdData.LogDebug}); err != nil {
		return err
	}

	if err := common.DockerRegistryInit(ctx, &getAutogeneratedValuedCmdData); err != nil {
		return err
	}

	if err := ssh_agent.Init(ctx, common.GetSSHKey(&getAutogeneratedValuedCmdData)); err != nil {
		return fmt.Errorf("cannot initialize ssh agent: %w", err)
	}
	defer func() {
		err := ssh_agent.Terminate()
		if err != nil {
			logboek.Error().LogF("WARNING: ssh agent termination failed: %s\n", err)
		}
	}()

	giterminismManager, err := common.GetGiterminismManager(ctx, &getAutogeneratedValuedCmdData)
	if err != nil {
		return err
	}

	_, werfConfig, err := common.GetRequiredWerfConfig(ctx, &getAutogeneratedValuedCmdData, giterminismManager, common.GetWerfConfigOptions(&getAutogeneratedValuedCmdData, false))
	if err != nil {
		return fmt.Errorf("unable to load werf config: %w", err)
	}
	if err := werfConfig.CheckThatImagesExist(imagesToProcess.OnlyImages); err != nil {
		return err
	}

	projectName := werfConfig.Meta.Project
	environment := *getAutogeneratedValuedCmdData.Environment

	namespace, err := deploy_params.GetKubernetesNamespace(*getAutogeneratedValuedCmdData.Namespace, environment, werfConfig)
	if err != nil {
		return err
	}

	var imagesRepository string
	var imagesInfoGetters []*image.InfoGetter
	imageNameList := common.GetImageNameList(imagesToProcess, werfConfig)

	if *getAutogeneratedValuedCmdData.StubTags {
		imagesInfoGetters = common.StubImageInfoGetters(werfConfig)
		imagesRepository = common.StubRepoAddress
	} else if len(imageNameList) > 0 {
		projectTmpDir, err := tmp_manager.CreateProjectDir(ctx)
		if err != nil {
			return fmt.Errorf("getting project tmp dir failed: %w", err)
		}
		defer tmp_manager.ReleaseProjectDir(projectTmpDir)

		_, err = getAutogeneratedValuedCmdData.Repo.GetAddress()
		if err != nil {
			return fmt.Errorf("%w (use --stub-tags option to get service values without real tags)", err)
		}
		stagesStorage, err := common.GetStagesStorage(ctx, containerBackend, &getAutogeneratedValuedCmdData)
		if err != nil {
			return err
		}
		finalStagesStorage, err := common.GetOptionalFinalStagesStorage(ctx, containerBackend, &getAutogeneratedValuedCmdData)
		if err != nil {
			return err
		}
		synchronization, err := common.GetSynchronization(ctx, &getAutogeneratedValuedCmdData, projectName, stagesStorage)
		if err != nil {
			return err
		}
		storageLockManager, err := common.GetStorageLockManager(ctx, synchronization)
		if err != nil {
			return err
		}
		secondaryStagesStorageList, err := common.GetSecondaryStagesStorageList(ctx, stagesStorage, containerBackend, &getAutogeneratedValuedCmdData)
		if err != nil {
			return err
		}
		cacheStagesStorageList, err := common.GetCacheStagesStorageList(ctx, containerBackend, &getAutogeneratedValuedCmdData)
		if err != nil {
			return err
		}
		useCustomTagFunc, err := common.GetUseCustomTagFunc(&getAutogeneratedValuedCmdData, giterminismManager, imageNameList)
		if err != nil {
			return err
		}

		storageManager := manager.NewStorageManager(projectName, stagesStorage, finalStagesStorage, secondaryStagesStorageList, cacheStagesStorageList, storageLockManager)

		conveyorOptions, err := common.GetConveyorOptions(ctx, &getAutogeneratedValuedCmdData, imagesToProcess)
		if err != nil {
			return err
		}

		conveyorWithRetry := build.NewConveyorWithRetryWrapper(werfConfig, giterminismManager, giterminismManager.ProjectDir(), projectTmpDir, ssh_agent.SSHAuthSock, containerBackend, storageManager, storageLockManager, conveyorOptions)
		defer conveyorWithRetry.Terminate()

		if err := conveyorWithRetry.WithRetryBlock(ctx, func(c *build.Conveyor) error {
			shouldBeBuiltOptions, err := common.GetShouldBeBuiltOptions(&getAutogeneratedValuedCmdData, imageNameList)
			if err != nil {
				return err
			}

			if err := c.ShouldBeBuilt(ctx, shouldBeBuiltOptions); err != nil {
				return err
			}

			imagesRepository = storageManager.StagesStorage.String()
			imagesInfoGetters, err = c.GetImageInfoGetters(image.InfoGetterOptions{CustomTagFunc: useCustomTagFunc})
			if err != nil {
				return err
			}

			return nil
		}); err != nil {
			return err
		}
	}

	headHash, err := giterminismManager.LocalGitRepo().HeadCommitHash(ctx)
	if err != nil {
		return fmt.Errorf("getting HEAD commit hash failed: %w", err)
	}

	headTime, err := giterminismManager.LocalGitRepo().HeadCommitTime(ctx)
	if err != nil {
		return fmt.Errorf("getting HEAD commit time failed: %w", err)
	}

	serviceValues, err := helpers.GetServiceValues(ctx, projectName, imagesRepository, imagesInfoGetters, helpers.ServiceValuesOptions{
		Namespace:  namespace,
		Env:        environment,
		CommitHash: headHash,
		CommitDate: headTime,
	})
	if err != nil {
		return fmt.Errorf("error creating service values: %w", err)
	}

	fmt.Printf("%s", util.DumpYaml(serviceValues))

	return nil
}
