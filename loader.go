package hot

type Loader[K comparable, V any] func(keys []K) (found map[K]V, err error)

type LoaderChain[K comparable, V any] []Loader[K, V]

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
			// a value that would be returned by many loader will be overwritten by the last loader
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
