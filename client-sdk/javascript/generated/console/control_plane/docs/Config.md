
# Config

Config schema

## Properties

Name | Type
------------ | -------------
`allowedCidrs` | Array&lt;string&gt;
`auditRetentionDays` | number
`mfaEnforced` | boolean
`sessionIdleTimeoutSeconds` | number
`sessionMaxLifetimeSeconds` | number

## Example

```typescript
import type { Config } from ''

// TODO: Update the object below with actual values
const example = {
  "allowedCidrs": null,
  "auditRetentionDays": null,
  "mfaEnforced": null,
  "sessionIdleTimeoutSeconds": null,
  "sessionMaxLifetimeSeconds": null,
} satisfies Config

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as Config
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


