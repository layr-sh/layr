
# SMSDispatcherConfig


## Properties

Name | Type
------------ | -------------
`driver` | string
`templates` | [SMSDispatcherTemplatesConfig](SMSDispatcherTemplatesConfig.md)
`twilio` | [SMSDispatcherTwilioConfig](SMSDispatcherTwilioConfig.md)
`webhook` | [SMSDispatcherWebhookConfig](SMSDispatcherWebhookConfig.md)

## Example

```typescript
import type { SMSDispatcherConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "driver": null,
  "templates": null,
  "twilio": null,
  "webhook": null,
} satisfies SMSDispatcherConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as SMSDispatcherConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


