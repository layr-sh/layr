
# StatsResponse

StatsResponse schema

## Properties

Name | Type
------------ | -------------
`buckets` | [Array&lt;BucketUsage&gt;](BucketUsage.md)
`contentTypes` | [Array&lt;ContentTypeUsage&gt;](ContentTypeUsage.md)
`databaseBuckets` | number
`databaseChunkBytes` | number
`databaseChunks` | number
`databaseObjects` | number
`databaseSizeBytes` | number
`lastUpdatedAt` | Date
`multipartParts` | number
`multipartSizeBytes` | number
`multipartUploads` | number
`privateBuckets` | number
`publicBuckets` | number
`s3Buckets` | number
`s3Objects` | number
`s3SizeBytes` | number
`totalBuckets` | number
`totalObjects` | number
`totalSizeBytes` | number

## Example

```typescript
import type { StatsResponse } from ''

// TODO: Update the object below with actual values
const example = {
  "buckets": null,
  "contentTypes": null,
  "databaseBuckets": null,
  "databaseChunkBytes": null,
  "databaseChunks": null,
  "databaseObjects": null,
  "databaseSizeBytes": null,
  "lastUpdatedAt": null,
  "multipartParts": null,
  "multipartSizeBytes": null,
  "multipartUploads": null,
  "privateBuckets": null,
  "publicBuckets": null,
  "s3Buckets": null,
  "s3Objects": null,
  "s3SizeBytes": null,
  "totalBuckets": null,
  "totalObjects": null,
  "totalSizeBytes": null,
} satisfies StatsResponse

console.log(example)

// Convert the instance to a JSON string
const exampleJSON: string = JSON.stringify(example)
console.log(exampleJSON)

// Parse the JSON string back to an object
const exampleParsed = JSON.parse(exampleJSON) as StatsResponse
console.log(exampleParsed)
```

[[Back to top]](#) [[Back to API list]](../README.md#api-endpoints) [[Back to Model list]](../README.md#models) [[Back to README]](../README.md)


