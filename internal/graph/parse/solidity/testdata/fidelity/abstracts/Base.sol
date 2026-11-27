// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title Base
/// @notice Shared governance state.
abstract contract Base {
    error NotMember(address who);
    error AlreadyPaused();

    event Paused(address indexed by);

    enum Status {
        None,
        Active,
        Closed
    }

    struct Proposal {
        uint256 id;
        address proposer;
        Status status;
    }

    uint256 public constant MAX_MEMBERS = 10;
    uint256 internal immutable deployedAt;
    bool public paused;
    uint256 public memberCount;
    mapping(address => bool) public members;
    mapping(uint256 => Proposal) internal proposals;

    modifier whenNotPaused() {
        if (paused) revert AlreadyPaused();
        _;
    }

    modifier onlyMember() {
        _checkMember(msg.sender);
        _;
    }

    constructor() {
        deployedAt = block.number;
    }

    function pause() external onlyMember whenNotPaused {
        paused = true;
        emit Paused(msg.sender);
    }

    function _checkMember(address who) internal view {
        if (!members[who]) revert NotMember(who);
    }

    function _execute(uint256 id) internal virtual returns (bool);

    function _onMemberAdded(address who) internal virtual {
        members[who] = true;
        memberCount += 1;
    }
}
