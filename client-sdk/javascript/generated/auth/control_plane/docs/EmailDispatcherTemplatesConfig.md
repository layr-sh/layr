
# EmailDispatcherTemplatesConfig


## Properties

Name | Type
------------ | -------------
`emailVerification` | [EmailDispatcherTemplateConfig](EmailDispatcherTemplateConfig.md)
`passwordReset` | [EmailDispatcherTemplateConfig](EmailDispatcherTemplateConfig.md)
`signInOtp` | [EmailDispatcherTemplateConfig](EmailDispatcherTemplateConfig.md)
`suspiciousActivity` | [EmailDispatcherTemplateConfig](EmailDispatcherTemplateConfig.md)

## Example

```typescript
import type { EmailDispatcherTemplatesConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "emailVerification": null,
  "passwordReset": null,
  "signInOtp": null,
  "suspiciousActivity": null,
} satisfies EmailDispatcherTemplatesConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as EmailDispatcherTemplatesConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


