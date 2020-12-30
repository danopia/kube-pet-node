
export interface NodeAllocation extends Record<string,unknown> {
  NodeKey: string;
  RouterKey: string;
  NodeName: string;
  NodeIP: string;
  PodNet: string;
}

export interface NetworkingConfig {
  NodeRange: string;
  PodRange: string;
  ServiceRange: string;
  WireguardMode?: "SelfProvision" | "Manual";
  CniNumber?: number;
  CniMtu?: number;
  Routers: NetworkingRouter[];
}

export interface NetworkingRouter {
  PublicKey: string;
  Endpoint: string;
  NodePool: string;
  PodPool: string;
  PodPrefixLen: number;
}
