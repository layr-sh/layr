
# EmailDispatcherConfig


## Properties

Name | Type
------------ | -------------
`driver` | string
`senderEmail` | string
`senderName` | string
`smtp` | [EmailDispatcherSMTPConfig](EmailDispatcherSMTPConfig.md)
`templates` | [EmailDispatcherTemplatesConfig](EmailDispatcherTemplatesConfig.md)
`webhook` | [EmailDispatcherWebhookConfig](EmailDispatcherWebhookConfig.md)

## Example

```typescript
import type { EmailDispatcherConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "driver": null,
  "senderEmail": null,
  "senderName": null,
  "smtp": null,
  "templates": null,
  "webhook": null,
} satisfies EmailDispatcherConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as EmailDispatcherConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


