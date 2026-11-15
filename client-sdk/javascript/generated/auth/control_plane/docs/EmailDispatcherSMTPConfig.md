
# EmailDispatcherSMTPConfig


## Properties

Name | Type
------------ | -------------
`host` | string
`insecureSkipVerify` | boolean
`password` | string
`passwordConfigured` | boolean
`port` | number
`tlsMode` | string
`username` | string

## Example

```typescript
import type { EmailDispatcherSMTPConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "host": null,
  "insecureSkipVerify": null,
  "password": null,
  "passwordConfigured": null,
  "port": null,
  "tlsMode": null,
  "username": null,
} satisfies EmailDispatcherSMTPConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as EmailDispatcherSMTPConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


