import { Reflector, ReflectorEvent, KindIdsReq } from "https://deno.land/x/kubernetes_client@v0.2.3/lib/reflector.ts";

import { fromIterable } from "https://deno.land/x/stream_observables@v1.0/sources/from-iterable.ts";
import { fromTimer } from "https://deno.land/x/stream_observables@v1.0/sources/from-timer.ts";
import { just } from "https://deno.land/x/stream_observables@v1.0/sources/just.ts";
import { merge } from "https://deno.land/x/stream_observables@v1.0/combiners/merge.ts";
import { combineLatest } from "https://deno.land/x/stream_observables@v1.0/combiners/combine-latest.ts";
// import { merge } from "https://deno.land/x/stream_observables@v1.0/combiners/merge.ts";
import { map } from "https://deno.land/x/stream_observables@v1.0/transforms/map.ts";
import { filter } from "https://deno.land/x/stream_observables@v1.0/transforms/filter.ts";
import { debounce } from "https://deno.land/x/stream_observables@v1.0/transforms/debounce.ts";
import { distinct } from "https://deno.land/x/stream_observables@v1.0/transforms/distinct.ts";

export {
  fromIterable,
  fromTimer,
  just,
  merge,
  combineLatest,
  // merge,
  map,
  filter,
  debounce,
  distinct,
};

// Lightly debounced function that emits the reflector's whole cache when things are somewhat calm
// Quite careful about only emitting snapshots that were fully synced
export function fromReflectorCache<T,S>(reflector: Reflector<T,S>, opts: {
  idleDelayMs?: number;
  eventFilter?: (evt: ReflectorEvent<T,S>) => boolean;
  changeFilterKeyFunc?: (node: T) => unknown,
} = {}) {
  return fromIterable(reflector.observeAll())
    .pipeThrough(filter(evt => {
      if (evt.type === 'BOOKMARK' || evt.type === 'DESYNCED' || evt.type === 'ERROR') return false;
      if (evt.type === 'MODIFIED' && opts.changeFilterKeyFunc) {
        const before = JSON.stringify(opts.changeFilterKeyFunc(evt.previous));
        const after = JSON.stringify(opts.changeFilterKeyFunc(evt.object));
        if (before === after) return false;
      }
      return opts.eventFilter ? opts.eventFilter(evt) : true;
    }))
    .pipeThrough(debounce(opts.idleDelayMs ?? 250))
    .pipeThrough(map(() => reflector.isSynced() ? Array.from(reflector.listCached()) : null))
    .pipeThrough(filter(x => x != null)) as ReadableStream<(T & KindIdsReq)[]>;
}
