// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

library L {
    struct Counter {
        uint256 value;
        mapping(address => bool) seen;
    }

    function f(uint256 a) internal pure returns (uint256) {
        return a;
    }

    function f(uint256 a, uint256 b) internal pure returns (uint256) {
        return a + b;
    }

    function inc(Counter storage c) internal {
        c.value += 1;
    }

    function inc(Counter storage c, uint256 by) internal {
        c.value += by;
    }

    function peek(Counter storage c) internal view returns (uint256) {
        return c.value;
    }
}

contract UsesL {
    using L for L.Counter;

    uint256 public constant START = 3;
    uint256 public total = START;
    L.Counter private y;

    function g(uint256 a) internal pure returns (uint256) {
        return a;
    }

    function g(uint256 a, uint256 b) internal pure returns (uint256) {
        return a * b;
    }

    function run() external returns (uint256) {
        uint256 one = L.f(3);
        uint256 two = L.f(1, 2);
        y.inc();
        y.inc(2);
        uint256 seen = y.peek();
        return g(one) + g(one, two) + seen;
    }
}
