// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract Root {
    uint256 internal rootValue;

    function f() public virtual {
        rootValue = 1;
    }
}

contract Left is Root {
    function f() public virtual override {
        rootValue = 2;
    }
}

contract Right is Root {
    mapping(address => uint256) internal rightMap;

    function f() public virtual override {
        rootValue = 3;
    }
}

// Solidity lists bases from the most base-like to the most derived, so the
// linearisation is Leaf, Right, Left, Root and `super.f()` runs Right.f.
contract Leaf is Root, Left, Right {
    uint256 internal leafValue;

    function f() public override(Root, Left, Right) {
        super.f();
        leafValue = rootValue;
    }
}
