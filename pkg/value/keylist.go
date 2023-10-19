package value

type KeyList struct {
	seen map[string]struct{}
	keys []string
}

func (k *KeyList) Get() []string {
	return k.keys
}

func (k *KeyList) Add(keys ...string) {
	if k.seen == nil {
		k.seen = map[string]struct{}{}
	}
	for _, key := range keys {
		if _, ok := k.seen[key]; !ok {
			k.seen[key] = struct{}{}
			k.keys = append(k.keys, key)
		}
	}
}
