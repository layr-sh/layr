import { LayrAPIError } from "layr-client";
import type { TasksControlPlane, TasksControlPlaneModels } from "layr-client";

export default function (tasks: TasksControlPlane) {
  const auth = useAuth();
  const busy = ref(false);
  const error = ref("");
  const notice = ref("");
  const config = ref<TasksControlPlaneModels.Config | null>(null);
  const stats = ref<TasksControlPlaneModels.GetStatsResponse | null>(null);
  const jobs = ref<TasksControlPlaneModels.ListJobsResponse | null>(null);
  const selectedJob = ref<TasksControlPlaneModels.GetJobResponse | null>(null);
  const executions = ref<TasksControlPlaneModels.ListExecutionsResponse | null>(null);
  const dlq = ref<TasksControlPlaneModels.ListDLQResponse | null>(null);
  const triggered = ref<TasksControlPlaneModels.TriggerExecutionResponse | null>(null);
  const jobFilter = ref("");
  const executionOffset = ref(0);
  const dlqOffset = ref(0);
  const limit = 10;

  async function run(operation: () => Promise<void>, message = "") {
    if (busy.value) return false;
    busy.value = true;
    error.value = "";
    notice.value = "";
    try {
      await operation();
      notice.value = message;
      return true;
    } catch (err) {
      error.value = err instanceof Error ? err.message : "The request failed. Please try again.";
      if (err instanceof LayrAPIError && (err.status === 401 || err.status === 403)) {
        await auth.refresh();
        if (!auth.state.value) await navigateTo("/sign-in");
      }
      return false;
    } finally {
      busy.value = false;
    }
  }
  function load(section: "statistics" | "configuration" | "jobs") {
    return run(async () => {
      if (section === "statistics") stats.value = await tasks.stats.get();
      if (section === "configuration") config.value = await tasks.config.get();
      if (section === "jobs") jobs.value = await tasks.jobs.list();
    });
  }
  function saveConfig(input: TasksControlPlaneModels.Config) {
    return run(async () => {
      config.value = await tasks.config.update(input);
    }, "Tasks Configuration Saved");
  }
  function selectJob(id: string) {
    return run(async () => {
      selectedJob.value = await tasks.jobs.get(id);
    });
  }
  function saveJob(
    form: Omit<TasksControlPlaneModels.CreateJobInput, "targetPayload"> & { targetPayload: string },
  ) {
    return run(async () => {
      const targetPayload: unknown = JSON.parse(form.targetPayload);
      if (!targetPayload || typeof targetPayload !== "object" || Array.isArray(targetPayload))
        throw new Error("Target payload must be a JSON object.");
      const input = {
        ...form,
        name: form.name.trim(),
        cronExpression: form.cronExpression.trim(),
        timezone: form.timezone?.trim(),
        targetPayload,
      };
      if (selectedJob.value) await tasks.jobs.update(selectedJob.value.id, input);
      else await tasks.jobs.create(input);
      jobs.value = await tasks.jobs.list();
    }, "Task Job Saved");
  }
  function deleteJob(id: string) {
    return run(async () => {
      await tasks.jobs.delete(id);
      selectedJob.value = null;
      jobs.value = await tasks.jobs.list();
    }, "Task Job Deleted");
  }
  function listExecutions(offset = 0) {
    return run(async () => {
      executions.value = await tasks.executions.list({
        limit,
        offset,
        jobId: jobFilter.value.trim() || undefined,
      });
      executionOffset.value = offset;
    });
  }
  function trigger(jobId: string, payload: string) {
    triggered.value = null;
    return run(async () => {
      const input: TasksControlPlaneModels.TriggerExecutionInput = {
        jobId: jobId.trim() || undefined,
      };
      if (payload.trim()) {
        const parsed: unknown = JSON.parse(payload);
        if (!parsed || typeof parsed !== "object" || Array.isArray(parsed))
          throw new Error("Execution payload must be a JSON object.");
        input.payload = parsed;
      }
      triggered.value = await tasks.executions.create(input);
    }, "Task Execution Queued");
  }
  function listDLQ(offset = 0) {
    return run(async () => {
      dlq.value = await tasks.dlq.list({ limit, offset });
      dlqOffset.value = offset;
    });
  }
  function retryDLQ(id: string) {
    return run(async () => {
      await tasks.dlq.retry(id);
      dlqOffset.value = 0;
      dlq.value = await tasks.dlq.list({ limit, offset: 0 });
    }, "Task Execution Requeued");
  }
  function purgeDLQ(id: string) {
    return run(async () => {
      await tasks.dlq.purge(id);
      dlqOffset.value = 0;
      dlq.value = await tasks.dlq.list({ limit, offset: 0 });
    }, "Dead-Lettered Execution Purged");
  }
  return {
    busy,
    error,
    notice,
    config,
    stats,
    jobs,
    selectedJob,
    executions,
    dlq,
    triggered,
    jobFilter,
    executionOffset,
    dlqOffset,
    limit,
    run,
    load,
    saveConfig,
    selectJob,
    saveJob,
    deleteJob,
    listExecutions,
    trigger,
    listDLQ,
    retryDLQ,
    purgeDLQ,
  };
}
