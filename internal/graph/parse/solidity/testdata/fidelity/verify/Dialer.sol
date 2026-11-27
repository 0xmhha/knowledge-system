// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import { IOracle } from "./Rules.sol";

interface ISink {
    function take(uint256 amount) external;
}

contract Dialer {
    struct Route {
        address hop;
        ISink sink;
    }

    address[] internal targets;
    mapping(uint256 => Route) internal routes;
    IOracle internal oracle;
    ISink internal sink;

    function paySender(uint256 amount) external {
        (bool ok, ) = msg.sender.call{value: amount}("");
        require(ok);
    }

    function fanOut(bytes calldata data) external {
        for (uint256 i = 0; i < targets.length; i++) {
            (bool ok, ) = targets[i].call(data);
            require(ok);
        }
    }

    function viaRoute(uint256 id, bytes calldata data) external {
        Route storage route = routes[id];
        (bool ok, ) = route.hop.delegatecall(data);
        require(ok);
    }

    function typed(uint256 amount, address asset) external returns (uint256) {
        sink.take(amount);
        oracle.set(amount);
        return oracle.price(asset);
    }

    function quiet(uint256 id) external view returns (uint256) {
        return routes[id].hop.code.length + targets.length;
    }
}
