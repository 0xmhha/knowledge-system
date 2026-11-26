// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

library Rules {
    struct Limits {
        uint256 low;
        uint256 high;
    }

    error OutOfRange(uint256 value);

    function within(Limits storage self, uint256 value) internal view returns (bool) {
        return value >= self.low && value <= self.high;
    }

    function scale(uint256 value) internal pure returns (uint256) {
        return value * 2;
    }
}

library Caps {
    struct Limits {
        uint256 cap;
    }
}

interface IOracle {
    function price(address asset) external view returns (uint256);

    function set(uint256 value) external;

    function set(address asset) external;
}
