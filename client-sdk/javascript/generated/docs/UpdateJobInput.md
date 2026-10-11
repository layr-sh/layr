
# UpdateJobInput

UpdateJobInput schema

## Properties

Name | Type
------------ | -------------
`cronExpression` | string
`isEnabled` | boolean
`name` | string
`targetPayload` | any
`targetType` | string
`timezone` | string

## Example

```typescript
import type { UpdateJobInput } from ''

// TODO: Update the object below with actual values
const example = {
  "cronExpression": null,
  "isEnabled": null,
  "name": null,
  "targetPayload": null,
  "targetType": null,
  "timezone": null,
} satisfies UpdateJobInput

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as UpdateJobInput
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


