// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract Pointers {
    struct Proposal {
        uint256 approved;
        bool done;
        address[] voters;
    }

    mapping(uint256 => Proposal) internal proposals;
    mapping(uint256 => address[]) internal lists;
    uint256 internal version;

    function vote(uint256 id) external {
        Proposal storage proposal = proposals[id];
        proposal.approved += 1;
        proposal.voters.push(msg.sender);
        address[] storage voters = proposal.voters;
        voters[0] = msg.sender;
        delete voters[0];
    }

    function _next() internal returns (address[] storage fresh, address[] storage old, uint256 next) {
        next = version + 1;
        old = lists[version];
        fresh = lists[next];
        version = next;
    }

    function _current() internal view returns (address[] storage) {
        return lists[version];
    }

    function grow(address member) external {
        (address[] storage fresh, address[] storage old, uint256 next) = _next();
        fresh.push(member);
        address[] storage current = _current();
        current.push(member);
        require(old.length < next);
    }

    function rebind(uint256 a, uint256 b) external {
        Proposal storage p = proposals[a];
        p = proposals[b];
        p.done = true;
    }

    function through(Proposal storage given) internal {
        given.done = true;
    }
}
