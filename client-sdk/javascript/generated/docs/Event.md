
# Event

Event schema

## Properties

Name | Type
------------ | -------------
`action` | string
`actor` | [EventActor](EventActor.md)
`context` | [EventContext](EventContext.md)
`createdAt` | Date
`data` | { [key: string]: any | null; }
`id` | any
`metadata` | { [key: string]: any | null; }
`resourceId` | string
`resourceType` | string
`type` | string

## Example

```typescript
import type { Event } from ''

// TODO: Update the object below with actual values
const example = {
  "action": null,
  "actor": null,
  "context": null,
  "createdAt": null,
  "data": null,
  "id": null,
  "metadata": null,
  "resourceId": null,
  "resourceType": null,
  "type": null,
} satisfies Event

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as Event
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


