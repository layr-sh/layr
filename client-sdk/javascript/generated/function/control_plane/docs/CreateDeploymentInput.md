
# CreateDeploymentInput

CreateDeploymentInput schema

## Properties

Name | Type
------------ | -------------
`bundleContent` | string
`bundleFiles` | { [key: string]: string; }
`bundleFormat` | string
`environmentVariables` | { [key: string]: string; }
`workerdRuntimeConfig` | [WorkerdRuntimeConfig](WorkerdRuntimeConfig.md)

## Example

```typescript
import type { CreateDeploymentInput } from ''

// TODO: Update the object below with actual values
const example = {
  "bundleContent": null,
  "bundleFiles": null,
  "bundleFormat": null,
  "environmentVariables": null,
  "workerdRuntimeConfig": null,
} satisfies CreateDeploymentInput

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as CreateDeploymentInput
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


