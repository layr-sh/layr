
# EventHookDelivery

EventHookDelivery schema

## Properties

Name | Type
------------ | -------------
`attemptCount` | number
`createdAt` | Date
`deliveredAt` | Date
`durationMs` | number
`errorMessage` | string
`eventHookId` | any
`eventId` | any
`eventType` | string
`httpResponseStatus` | number
`id` | any
`isDelivered` | boolean
`payload` | any
`result` | string

## Example

```typescript
import type { EventHookDelivery } from ''

// TODO: Update the object below with actual values
const example = {
  "attemptCount": null,
  "createdAt": null,
  "deliveredAt": null,
  "durationMs": null,
  "errorMessage": null,
  "eventHookId": null,
  "eventId": null,
  "eventType": null,
  "httpResponseStatus": null,
  "id": null,
  "isDelivered": null,
  "payload": null,
  "result": null,
} satisfies EventHookDelivery

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as EventHookDelivery
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


