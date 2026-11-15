
# MFAConfig


## Properties

Name | Type
------------ | -------------
`enabled` | boolean
`issuer` | string
`policy` | string
`riskTriggers` | Array&lt;string&gt;

## Example

```typescript
import type { MFAConfig } from ''

// TODO: Update the object below with actual values
const example = {
  "enabled": null,
  "issuer": null,
  "policy": null,
  "riskTriggers": null,
} satisfies MFAConfig

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as MFAConfig
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


