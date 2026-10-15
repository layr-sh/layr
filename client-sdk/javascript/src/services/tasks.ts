import type { Configuration } from '../../generated/runtime';
import {
  ConfigApi,
  DLQApi,
  ExecutionsApi,
  JobsApi,
  StatsApi,
  type TasksDlqListRequest,
  type TasksExecutionsListRequest,
} from '../../generated/apis';
import type {
  Config,
  CreateJobInput,
  GetJobResponse,
  GetStatsResponse,
  Job,
  ListDLQResponse,
  ListExecutionsResponse,
  ListJobsResponse,
  RetryDLQResponse,
  TriggerExecutionInput,
  TriggerExecutionResponse,
  UpdateJobInput,
} from '../../generated/models';
import { ATTACH_CONTROL_PLANE, type ElevatableService, type LayrClient } from '../core/client';
import { handleApiCall } from '../core/transport';

export interface TasksControlPlane {
  config: {
    get: () => Promise<Config>;
    update: (input: Config) => Promise<Config>;
  };
  dlq: {
    list: (params?: TasksDlqListRequest) => Promise<ListDLQResponse>;
    purge: (executionId: string) => Promise<void>;
    retry: (executionId: string) => Promise<RetryDLQResponse>;
  };
  executions: {
    create: (input: TriggerExecutionInput) => Promise<TriggerExecutionResponse>;
    list: (params?: TasksExecutionsListRequest) => Promise<ListExecutionsResponse>;
  };
  jobs: {
    create: (input: CreateJobInput) => Promise<Job>;
    delete: (jobId: string) => Promise<void>;
    get: (jobId: string) => Promise<GetJobResponse>;
    list: () => Promise<ListJobsResponse>;
    update: (jobId: string, input: UpdateJobInput) => Promise<Job>;
  };
  stats: {
    get: () => Promise<GetStatsResponse>;
  };
}

export interface TasksService extends ElevatableService<TasksControlPlane> {
  client: LayrClient;
  [ATTACH_CONTROL_PLANE]?: (elevatedConfig: Configuration) => TasksControlPlane;
}

export function useTasks(client: LayrClient): TasksService {
  const service: TasksService = {
    client,

    [ATTACH_CONTROL_PLANE]: (elevatedConfig: Configuration): TasksControlPlane => ({
      config: {
        get: () => handleApiCall(() => new ConfigApi(elevatedConfig).tasksConfigGet()),
        update: (input: Config) =>
          handleApiCall(() => new ConfigApi(elevatedConfig).tasksConfigUpdate({ config: input })),
      },
      dlq: {
        list: (params = {}) =>
          handleApiCall(() => new DLQApi(elevatedConfig).tasksDlqList(params)),
        purge: (executionId: string) =>
          handleApiCall(() => new DLQApi(elevatedConfig).tasksDlqPurge({ executionId })),
        retry: (executionId: string) =>
          handleApiCall(() =>
            new DLQApi(elevatedConfig).tasksDlqRetry({ executionId, body: {} })
          ),
      },
      executions: {
        create: (input: TriggerExecutionInput) =>
          handleApiCall(() =>
            new ExecutionsApi(elevatedConfig).tasksExecutionsCreate({
              triggerExecutionInput: input,
            })
          ),
        list: (params = {}) =>
          handleApiCall(() => new ExecutionsApi(elevatedConfig).tasksExecutionsList(params)),
      },
      jobs: {
        create: (input: CreateJobInput) =>
          handleApiCall(() =>
            new JobsApi(elevatedConfig).tasksJobsCreate({ createJobInput: input })
          ),
        delete: (jobId: string) =>
          handleApiCall(() => new JobsApi(elevatedConfig).tasksJobsDelete({ jobId })),
        get: (jobId: string) =>
          handleApiCall(() => new JobsApi(elevatedConfig).tasksJobsGet({ jobId })),
        list: () => handleApiCall(() => new JobsApi(elevatedConfig).tasksJobsList()),
        update: (jobId: string, input: UpdateJobInput) =>
          handleApiCall(() =>
            new JobsApi(elevatedConfig).tasksJobsUpdate({
              jobId,
              updateJobInput: input,
            })
          ),
      },
      stats: {
        get: () => handleApiCall(() => new StatsApi(elevatedConfig).tasksStatsGet()),
      },
    }),
  };

  return service;
}
