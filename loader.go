package hot

// Loader is a function type that loads values for the given keys.
// It should return a map of found key-value pairs and an error if the operation fails.
// Keys that cannot be found should not be included in the returned map.
type Loader[K comparable, V any] func(keys []K) (found map[K]V, err error)

// LoaderChain is a slice of loaders that are executed in sequence.
// Each loader is called with the keys that were not found by previous loaders.
type LoaderChain[K comparable, V any] []Loader[K, V]

// run executes the loader chain with the given missing keys.
// It returns found values, still missing keys, and an error if any loader fails.
// If a loader returns an error, the entire operation fails and no values are returned.
// Values returned by later loaders in the chain will overwrite values from earlier loaders.
func (loaders LoaderChain[K, V]) run(missing []K) (results map[K]V, other []K, err error) {
	results = map[K]V{}

	stillMissing := map[K]struct{}{}
	for _, key := range missing {
		stillMissing[key] = struct{}{}
	}

	for i := range loaders {
		count := len(stillMissing)
		if count == 0 {
			break
		}

		toFetch := make([]K, 0, count)
		for key := range stillMissing {
			toFetch = append(toFetch, key)
		}

		found, err := loaders[i](toFetch)
		if err != nil {
			return map[K]V{}, []K{}, err
		}

		for k, v := range found {
			// A value that would be returned by many loaders will be overwritten by the last loader
			results[k] = v
			delete(stillMissing, k)
		}
	}

	missing = make([]K, 0, len(stillMissing))
	for key := range stillMissing {
		missing = append(missing, key)
	}

	return results, missing, nil
}
