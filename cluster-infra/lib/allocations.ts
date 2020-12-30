import { Address4, BigInteger } from "../deps.ts";
import { NetworkingConfig, NodeAllocation } from "../types.ts";

export function findNextAllocation(netConf: NetworkingConfig, allocations: NodeAllocation[]): NodeAllocation {

  const allNodeIps = allocations.map(x => new Address4(x.NodeIP));
  const allPodNets = allocations.map(x => new Address4(x.PodNet));

  for (const router of netConf.Routers) {

    const nodeWalker = new IpWalker(router.NodePool, 32);
    while (nodeWalker.next()) {
      if (allNodeIps.some(x => x.isInSubnet(nodeWalker.cursor) || nodeWalker.cursor.isInSubnet(x))) continue;

      const podWalker = new IpWalker(router.PodPool, router.PodPrefixLen);
      while (podWalker.next()) {
        if (allPodNets.some(x => x.isInSubnet(podWalker.cursor) || podWalker.cursor.isInSubnet(x))) continue;
        // console.log(nodeWalker.cursor.address, podWalker.cursor.address);

        return {
          NodeIP: nodeWalker.cursor.addressMinusSuffix,
          PodNet: podWalker.cursor.address,
          RouterKey: router.PublicKey,
          NodeKey: '',
          NodeName: '',
        };
      }

      break; // if we got here, there was a node opening but not a pod opening, so go to next router (if any)
    }
  }

  throw new Error(`Failed to locate unallocated address space in any available router`);
}

export class IpWalker {
  constructor(pool: string, public prefixLen: number) {
    this.pool = new Address4(pool);
    this.increment = new BigInteger((1 << (32 - prefixLen)).toFixed(0));

    this.cursor = new Address4(this.pool.addressMinusSuffix + `/${this.prefixLen}`);
  }
  pool: any;
  increment: any;
  cursor: any;

  next() {
    const nextNum = this.cursor.bigInteger().add(this.increment);
    const nextAddr = Address4.fromBigInteger(nextNum).addressMinusSuffix;
    this.cursor = new Address4(`${nextAddr}/${this.prefixLen}`);
    return this.cursor.isInSubnet(this.pool) && !this.pool.endAddress().isInSubnet(this.cursor);
  }
}
