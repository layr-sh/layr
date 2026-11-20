import { LayrAPIError } from "layr-client";
import type {
  FileStorageService,
  FileStorageControlPlane,
  FileStorageControlPlaneModels,
  FileStorageBaseModels,
} from "layr-client";

export default function (storage: FileStorageService & { cp: FileStorageControlPlane }) {
  const auth = useAuth();
  const busy = ref(false);
  const error = ref("");
  const notice = ref("");
  const stats = ref<FileStorageControlPlaneModels.StatsResponse | null>(null);
  const buckets = ref<FileStorageControlPlaneModels.ListBucketsResponse | null>(null);
  const bucket = ref<FileStorageControlPlaneModels.Bucket | null>(null);
  const objects = ref<FileStorageControlPlaneModels.ListBucketObjectsResponse | null>(null);
  const config = ref<FileStorageControlPlaneModels.Config | null>(null);
  const signedUrl = ref<FileStorageBaseModels.PresignURLResponse | null>(null);
  const offset = ref(0);
  const limit = 20;
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
  async function fetchBuckets() {
    buckets.value = await storage.cp.buckets.list();
  }
  function load(section: "buckets" | "configuration" | "statistics") {
    return run(async () => {
      if (section === "buckets") await fetchBuckets();
      else if (section === "statistics") stats.value = await storage.cp.stats.get();
      else config.value = await storage.cp.config.get();
    });
  }
  function saveConfig(input: FileStorageControlPlaneModels.Config) {
    return run(async () => {
      config.value = await storage.cp.config.update(input);
    }, "File Storage Configuration Saved");
  }
  function getBucket(name: string) {
    bucket.value = null;
    return run(async () => {
      bucket.value = await storage.cp.buckets.get(name);
    });
  }
  function saveBucket(input: FileStorageControlPlaneModels.CreateBucketInput, name: string) {
    return run(
      async () => {
        if (name) bucket.value = await storage.cp.buckets.update(name, input);
        else bucket.value = await storage.cp.buckets.create(input);
        await fetchBuckets();
      },
      name ? "File Storage Bucket Updated" : "File Storage Bucket Created",
    );
  }
  function deleteBucket(name: string) {
    return run(async () => {
      await storage.cp.buckets.delete(name);
      bucket.value = null;
      await fetchBuckets();
    }, "File Storage Bucket Deleted");
  }
  function listObjects(name: string, nextOffset = 0) {
    return run(async () => {
      objects.value = await storage.cp.buckets.list_objects(name, { limit, offset: nextOffset });
      offset.value = nextOffset;
    });
  }
  async function selectBucket(name: string) {
    bucket.value = null;
    objects.value = null;
    offset.value = 0;
    if (!name) return false;
    return run(async () => {
      const nextBucket = await storage.cp.buckets.get(name);
      const nextObjects = await storage.cp.buckets.list_objects(name, { limit, offset: 0 });
      bucket.value = nextBucket;
      objects.value = nextObjects;
    });
  }
  function upload(name: string, key: string, file: File, contentType: string) {
    return run(async () => {
      await storage.objects.upload_put(name, key, file, { contentType });
      objects.value = await storage.cp.buckets.list_objects(name, { limit, offset: 0 });
      offset.value = 0;
    }, "File Storage Object Uploaded");
  }
  function deleteObject(name: string, key: string) {
    return run(async () => {
      await storage.objects.delete(name, key);
      const nextOffset = Math.max(
        0,
        offset.value - (objects.value?.data?.length === 1 ? limit : 0),
      );
      objects.value = await storage.cp.buckets.list_objects(name, { limit, offset: nextOffset });
      offset.value = nextOffset;
    }, "File Storage Object Deleted");
  }
  function download(name: string, key: string) {
    return run(async () => {
      const file = await storage.objects.download(name, key);
      const objectUrl = URL.createObjectURL(file);
      const anchorElement = document.createElement("a");
      anchorElement.href = objectUrl;
      anchorElement.download = key.split("/").at(-1)!;
      anchorElement.click();
      setTimeout(() => URL.revokeObjectURL(objectUrl), 0);
    }, "File Storage Download Started");
  }
  function sign(input: FileStorageBaseModels.PresignURLInput) {
    signedUrl.value = null;
    return run(async () => {
      signedUrl.value = await storage.url.presign(input);
    }, "File Storage URL Signed");
  }
  return {
    busy,
    error,
    notice,
    stats,
    buckets,
    bucket,
    objects,
    config,
    signedUrl,
    offset,
    limit,
    run,
    load,
    saveConfig,
    getBucket,
    saveBucket,
    deleteBucket,
    listObjects,
    selectBucket,
    upload,
    deleteObject,
    download,
    sign,
  };
}
