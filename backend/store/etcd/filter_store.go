package etcd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/coreos/etcd/clientv3"
	"github.com/sensu/sensu-go/types"
)

var (
	eventFiltersPathPrefix = "event-filters"
	eventFilterKeyBuilder  = newKeyBuilder(eventFiltersPathPrefix)
)

func getEventFilterPath(filter *types.EventFilter) string {
	return eventFilterKeyBuilder.withResource(filter).build(filter.Name)
}

func getEventFiltersPath(ctx context.Context, name string) string {
	return eventFilterKeyBuilder.withContext(ctx).build(name)
}

func (s *etcdStore) DeleteEventFilterByName(ctx context.Context, name string) error {
	if name == "" {
		return errors.New("must specify name of filter")
	}

	resp, err := s.kvc.Delete(ctx, getEventFiltersPath(ctx, name))
	if err != nil {
		return err
	}

	if resp.Deleted != 1 {
		return fmt.Errorf("filter %s does not exist", name)
	}

	return nil
}

// GetEventFilters gets the list of filters for an (optional) organization. Passing
// the empty string as the org will return all filters.
func (s *etcdStore) GetEventFilters(ctx context.Context) ([]*types.EventFilter, error) {
	resp, err := query(ctx, s, getEventFiltersPath)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return []*types.EventFilter{}, nil
	}

	filtersArray := make([]*types.EventFilter, len(resp.Kvs))
	for i, kv := range resp.Kvs {
		filter := &types.EventFilter{}
		err = json.Unmarshal(kv.Value, filter)
		if err != nil {
			return nil, err
		}
		filtersArray[i] = filter
	}

	return filtersArray, nil
}

func (s *etcdStore) GetEventFilterByName(ctx context.Context, name string) (*types.EventFilter, error) {
	if name == "" {
		return nil, errors.New("must specify name of filter")
	}

	resp, err := s.kvc.Get(ctx, getEventFiltersPath(ctx, name))
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}

	filterBytes := resp.Kvs[0].Value
	filter := &types.EventFilter{}
	if err := json.Unmarshal(filterBytes, filter); err != nil {
		return nil, err
	}

	return filter, nil
}

func (s *etcdStore) UpdateEventFilter(ctx context.Context, filter *types.EventFilter) error {
	if err := filter.Validate(); err != nil {
		return err
	}

	filterBytes, err := json.Marshal(filter)
	if err != nil {
		return err
	}

	cmp := clientv3.Compare(clientv3.Version(getEnvironmentsPath(filter.Organization, filter.Environment)), ">", 0)
	req := clientv3.OpPut(getEventFilterPath(filter), string(filterBytes))
	res, err := s.kvc.Txn(ctx).If(cmp).Then(req).Commit()
	if err != nil {
		return err
	}
	if !res.Succeeded {
		return fmt.Errorf(
			"could not create the filter %s in environment %s/%s",
			filter.Name,
			filter.Organization,
			filter.Environment,
		)
	}

	return nil
}