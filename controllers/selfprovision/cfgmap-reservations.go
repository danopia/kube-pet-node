package selfprovision

// func (r *ReservationList) findNextOpening(netCfg *Networking) (net.IP, *net.IPNet, error) {
// 	for _, router := range netCfg.WireguardRouters {

// 		nodeIP := make(net.IP, len(router.NodePoolStart))
// 		copy(nodeIP, router.NodePoolStart)

// 		for router.NodePool.Contains(nodeIP) {
// 			log.Println("checking node", nodeIP.String())

// 			// check if anyone's using it
// 			inUse := false
// 			for _, reserv := range r.reservations {
// 				if reserv.NodeIP.Equal(nodeIP) {
// 					inUse = true
// 				}
// 			}
// 			if !inUse {
// 				podOpening, err := r.findNextPodOpening(netCfg, router)
// 				if err == nil {
// 					return nodeIP, podOpening, nil
// 				}
// 			}

// 			incIP(nodeIP)
// 		}

// 	}
// 	return nil, nil, fmt.Errorf("No open NodeIP slots found")
// }

// func (r *ReservationList) findNextPodOpening(netCfg *Networking, router WireguardRouter) (*net.IPNet, error) {
// 	_, podNet, err := net.ParseCIDR(router.PodPool.IP.String() + "/" + strconv.Itoa(router.PodPrefixLen))
// 	if err != nil {
// 		return nil, err
// 	}

// 	for router.PodPool.Contains(podNet.IP) {
// 		log.Println("checking pod", podNet.String())

// 		// check if anyone's using it
// 		inUse := false
// 		for _, reserv := range r.reservations {
// 			if podNet.Contains(reserv.PodRange.IP) || reserv.PodRange.Contains(podNet.IP) {
// 				inUse = true
// 			}
// 		}
// 		if !inUse {
// 			return podNet, nil
// 		}

// 		_, myCopy, err := net.ParseCIDR(podNet.String())
// 		if err != nil {
// 			return nil, err
// 		}

// 		for myCopy.Contains(podNet.IP) {
// 			incIP(podNet.IP)
// 		}
// 	}

// 	return nil, fmt.Errorf("No open NodeIP slots found")
// }

// // https://stackoverflow.com/questions/29732128/copy-net-ip-in-golang
// func incIP(ip net.IP) {
// 	for j := len(ip) - 1; j >= 0; j-- {
// 		ip[j]++
// 		if ip[j] > 0 {
// 			break
// 		}
// 	}
// }
