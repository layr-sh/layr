
# Deployment


## Properties

Name | Type
------------ | -------------
`bundleFiles` | { [key: string]: any | null; }
`bundleFormat` | string
`bundleHash` | string
`createdAt` | Date
`endpointId` | any
`environmentVariables` | { [key: string]: string; }
`id` | any
`status` | string
`version` | number
`workerdRuntimeConfig` | [WorkerdRuntimeConfig](WorkerdRuntimeConfig.md)

## Example

```typescript
import type { Deployment } from ''

// TODO: Update the object below with actual values
const example = {
  "bundleFiles": null,
  "bundleFormat": null,
  "bundleHash": null,
  "createdAt": null,
  "endpointId": null,
  "environmentVariables": null,
  "id": null,
  "status": null,
  "version": null,
  "workerdRuntimeConfig": null,
} satisfies Deployment

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as Deployment
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


