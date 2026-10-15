import { describe, it } from "vitest";
import { RuleTester } from "oxlint/plugins-dev";
import representTypeName from "../represent-type-name.ts";

RuleTester.describe = describe;
RuleTester.it = it;

const ruleTester = new RuleTester({
  languageOptions: {
    parserOptions: {
      lang: "ts",
    },
  },
});

ruleTester.run("represent-type-name", representTypeName, {
  valid: [
    // Direct canonical name
    { code: "const runtimeConfig = useRuntimeConfig();" },
    // Suffixed with TypeName
    { code: "const abcRuntimeConfig = useRuntimeConfig();" },
    { code: "const appRuntimeConfig = useRuntimeConfig();" },
    { code: "const nitroRuntimeConfig = resolveRuntimeConfig(event);" },

    // Composable type inference
    { code: "const route = useRoute();" },
    { code: "const currentRoute = useRoute();" },
    { code: "const router = useRouter();" },
    { code: "const appRouter = useRouter();" },
    { code: "const appConfig = useAppConfig();" },

    // Constructor call (NewExpression)
    { code: "const queueManager = new QueueManager();" },
    { code: "const emailQueueManager = new QueueManager();" },
    { code: "const s3Client = new S3Client();" },
    { code: "const backupS3Client = new S3Client();" },

    // Factory function calls
    { code: "const openai = createOpenAI();" },
    { code: "const openAI = createOpenAI();" },
    { code: "const customOpenAI = createOpenAI();" },

    // Explicit type annotations
    { code: "const queueManager: QueueManager = getQueue();" },
    { code: "const taskQueueManager: QueueManager = getQueue();" },

    // Destructuring is ignored (property bindings)
    { code: "const { public: publicConfig } = useRuntimeConfig();" },
    { code: "const [firstItem, secondItem] = getItems();" },

    // Excluded / generic reactive state helpers
    { code: "const count = ref(0);" },
    { code: "const isVisible = useState('visible', () => false);" },
    { code: "const data = useFetch('/api');" },
    { code: "const cookie = useCookie('token');" },
    { code: "const total: number = 100;" },
    { code: "const message: string = 'hello';" },

    // Ignored / exceptions
    { code: "const _ = useRuntimeConfig();" },
    {
      code: "const config = useRuntimeConfig();",
      options: [{ exceptions: ["config"] }],
    },
  ],
  invalid: [
    {
      code: "const xxx = useRuntimeConfig();",
      errors: [
        {
          message: "'xxx' should be named 'runtimeConfig' or suffixed with 'RuntimeConfig'",
        },
      ],
    },
    {
      code: "const config = useRuntimeConfig();",
      errors: [
        {
          message: "'config' should be named 'runtimeConfig' or suffixed with 'RuntimeConfig'",
        },
      ],
    },
    {
      code: "const config = resolveRuntimeConfig(event);",
      errors: [
        {
          message: "'config' should be named 'runtimeConfig' or suffixed with 'RuntimeConfig'",
        },
      ],
    },
    {
      code: "const r = useRoute();",
      errors: [
        {
          message: "'r' should be named 'route' or suffixed with 'Route'",
        },
      ],
    },
    {
      code: "const r = useRouter();",
      errors: [
        {
          message: "'r' should be named 'router' or suffixed with 'Router'",
        },
      ],
    },
    {
      code: "const client = createOpenAI();",
      errors: [
        {
          message: "'client' should be named 'openAI' or suffixed with 'OpenAI'",
        },
      ],
    },
    {
      code: "const queue = new QueueManager();",
      errors: [
        {
          message: "'queue' should be named 'queueManager' or suffixed with 'QueueManager'",
        },
      ],
    },
    {
      code: "const client = new S3Client();",
      errors: [
        {
          message: "'client' should be named 's3Client' or suffixed with 'S3Client'",
        },
      ],
    },
    {
      code: "const manager: QueueManager = getQueue();",
      errors: [
        {
          message: "'manager' should be named 'queueManager' or suffixed with 'QueueManager'",
        },
      ],
    },
  ],
});
