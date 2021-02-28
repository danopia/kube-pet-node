import { Reflector, ReflectorEvent, KindIdsReq } from "https://deno.land/x/kubernetes_client@v0.2.0/lib/reflector.ts";

// Not actually from OWS, but fits in really well, sue me
import {
  readableStreamFromAsyncIterator as fromAsyncIterator,
} from "https://deno.land/std@0.88.0/io/streams.ts";

import { fromTimer } from "https://cloudydeno.github.io/observables-with-streams/src/sources/from-timer.ts";
import { just } from "https://cloudydeno.github.io/observables-with-streams/src/sources/just.ts";
import { merge } from "https://cloudydeno.github.io/observables-with-streams/src/combiners/merge.ts";
import { combineLatest } from "https://cloudydeno.github.io/observables-with-streams/src/combiners/combine-latest.ts";
// import { merge } from "https://cloudydeno.github.io/observables-with-streams/src/combiners/merge.ts";
import { map } from "https://cloudydeno.github.io/observables-with-streams/src/transforms/map.ts";
import { filter } from "https://cloudydeno.github.io/observables-with-streams/src/transforms/filter.ts";
import { debounce } from "https://cloudydeno.github.io/observables-with-streams/src/transforms/debounce.ts";
import { distinct } from "https://cloudydeno.github.io/observables-with-streams/src/transforms/distinct.ts";

export {
  fromAsyncIterator,
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
  return fromAsyncIterator(reflector.observeAll())
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
