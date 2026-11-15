
# SMSDispatcherTemplatesConfig


## Properties

Name | Type
------------ | -------------
`passwordReset` | [SMSDispatcherTemplateConfig](SMSDispatcherTemplateConfig.md)
`phoneVerification` | [SMSDispatcherTemplateConfig](SMSDispatcherTemplateConfig.md)
`signInOtp` | [SMSDispatcherTemplateConfig](SMSDispatcherTemplateConfig.md)

## Example

```typescript
import type { SMSDispatcherTemplatesConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "passwordReset": null,
  "phoneVerification": null,
  "signInOtp": null,
} satisfies SMSDispatcherTemplatesConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as SMSDispatcherTemplatesConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


