// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

library SetLib {
    struct Set {
        address[] values;
        mapping(address => uint256) positions;
    }

    function contains(Set storage set, address value) internal view returns (bool) {
        return set.positions[value] != 0;
    }

    function add(Set storage set, address value) internal returns (bool) {
        if (contains(set, value)) return false;
        set.values.push(value);
        set.positions[value] = set.values.length;
        return true;
    }

    function hash(address value) internal pure returns (bytes32) {
        return keccak256(abi.encodePacked(value));
    }
}
