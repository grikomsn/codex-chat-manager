package session

import (
	"sort"
	"time"
)

type projectBucket struct {
	key       string
	label     string
	groups    []SessionGroup
	bestRank  int
	newestAgg time.Time
}

func orderGroupsByProjectStatusRecency(groups []SessionGroup) []SessionGroup {
	if len(groups) <= 1 {
		return groups
	}

	bucketsByKey := make(map[string]*projectBucket, len(groups))
	for _, group := range groups {
		key, label := projectKeyLabel(group)
		bucket := bucketsByKey[key]
		if bucket == nil {
			bucket = &projectBucket{
				key:      key,
				label:    label,
				bestRank: statusRank(group.Status),
			}
			bucketsByKey[key] = bucket
		}
		bucket.groups = append(bucket.groups, group)
		rank := statusRank(group.Status)
		if rank < bucket.bestRank {
			bucket.bestRank = rank
		}
		if group.AggregateAt.After(bucket.newestAgg) {
			bucket.newestAgg = group.AggregateAt
		}
	}

	buckets := make([]*projectBucket, 0, len(bucketsByKey))
	for _, bucket := range bucketsByKey {
		sort.Slice(bucket.groups, func(i, j int) bool {
			ri := statusRank(bucket.groups[i].Status)
			rj := statusRank(bucket.groups[j].Status)
			if ri != rj {
				return ri < rj
			}
			if bucket.groups[i].AggregateAt.Equal(bucket.groups[j].AggregateAt) {
				return bucket.groups[i].Parent.ID > bucket.groups[j].Parent.ID
			}
			return bucket.groups[i].AggregateAt.After(bucket.groups[j].AggregateAt)
		})
		buckets = append(buckets, bucket)
	}

	sort.Slice(buckets, func(i, j int) bool {
		if buckets[i].bestRank != buckets[j].bestRank {
			return buckets[i].bestRank < buckets[j].bestRank
		}
		if !buckets[i].newestAgg.Equal(buckets[j].newestAgg) {
			return buckets[i].newestAgg.After(buckets[j].newestAgg)
		}
		if buckets[i].label != buckets[j].label {
			return buckets[i].label < buckets[j].label
		}
		return buckets[i].key < buckets[j].key
	})

	ordered := make([]SessionGroup, 0, len(groups))
	for _, bucket := range buckets {
		ordered = append(ordered, bucket.groups...)
	}
	return ordered
}

func projectKeyLabel(group SessionGroup) (string, string) {
	key := group.Parent.ProjectKey
	label := group.Parent.Project
	if label == "" {
		label = "unknown"
	}
	if key == "" {
		// Should be populated during snapshot load; fall back to the display label.
		key = label
	}
	return key, label
}

func statusRank(status Status) int {
	switch status {
	case StatusActive:
		return 0
	case StatusMixed:
		return 1
	case StatusArchived:
		return 2
	default:
		return 3
	}
}
