
# Endpoint


## Properties

Name | Type
------------ | -------------
`activeDeploymentId` | any
`createdAt` | Date
`description` | string
`entrypoint` | string
`id` | any
`isPublic` | boolean
`memoryLimitMb` | number
`name` | string
`runtime` | string
`timeoutSeconds` | number
`updatedAt` | Date

## Example

```typescript
import type { Endpoint } from ''

// TODO: Update the object below with actual values
const example = {
  "activeDeploymentId": null,
  "createdAt": null,
  "description": null,
  "entrypoint": null,
  "id": null,
  "isPublic": null,
  "memoryLimitMb": null,
  "name": null,
  "runtime": null,
  "timeoutSeconds": null,
  "updatedAt": null,
} satisfies Endpoint

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as Endpoint
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


