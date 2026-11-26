// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

interface ITarget {
    function poke(address who) external;

    function peek(address who) external view returns (bool);
}

contract LowLevel {
    address internal target;

    function send(address who) external {
        (bool ok, ) = target.call(abi.encodeCall(ITarget.poke, (who)));
        require(ok);
    }

    function look(address who) external view returns (bool) {
        (bool ok, bytes memory out) = target.staticcall(abi.encodeWithSelector(ITarget.peek.selector, who));
        return ok && out.length > 0;
    }

    function refund(uint256 amount) external {
        (bool ok, ) = msg.sender.call{value: amount}("");
        require(ok);
    }

    function opaque(bytes calldata data) external {
        (bool ok, ) = target.call(data);
        require(ok);
    }
}
