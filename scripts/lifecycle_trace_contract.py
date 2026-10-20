"""Check finite observation-hook prefixes, never universal or independent approval."""

OWNER = {
    'install', 'before_delete', 'after_delete', 'ack_close_begin', 'ack_close_end',
    *('send_' + edge + '_' + kind for edge in ('begin', 'end')
      for kind in ('progress', 'done_false', 'install_false')),
}
METHOD = {'method_entry', 'method_status_received_after_io',
          'method_ack_observed', 'method_return_defer'}


def validate_prefix(rows, candidate):
    members, deletion, ack, methods = set(), {}, {}, {}
    pending = None
    last_time = -1
    for index, row in enumerate(rows, 1):
        if row['sequence'] != index or row['candidate'] != candidate:
            raise ValueError('lifecycle identity/sequence differs')
        if row['elapsed_ns'] < last_time:
            raise ValueError('lifecycle clock order differs')
        last_time = row['elapsed_ns']
        kind, channel = row['event'], row['channel_id']
        if not isinstance(channel, int) or isinstance(channel, bool) or channel < 1:
            raise ValueError('invalid observed channel identity')
        if kind in OWNER:
            if row['map_observed'] is not True:
                raise ValueError('owner map observation missing')
            if pending is not None and not kind.startswith('send_end_'):
                raise ValueError('owner advanced while its synchronous send was pending')
            if kind == 'install':
                if channel in members or channel in deletion:
                    raise ValueError('duplicate/reused owned channel install')
                members.add(channel)
            elif kind == 'before_delete':
                if channel not in members or channel in deletion:
                    raise ValueError('delete did not start from installed membership')
                deletion[channel] = 'begun'
            elif kind == 'after_delete':
                if deletion.get(channel) != 'begun' or channel not in members:
                    raise ValueError('map deletion order differs')
                members.remove(channel)
                deletion[channel] = 'finished'
            elif kind == 'ack_close_begin':
                if deletion.get(channel) != 'finished' or channel in members or channel in ack:
                    raise ValueError('ack began before actual observed map deletion')
                ack[channel] = 'begun'
            elif kind == 'ack_close_end':
                if ack.get(channel) != 'begun':
                    raise ValueError('ack-close completion lacks its beginning')
                ack[channel] = 'finished'
            elif kind.startswith('send_begin_'):
                if channel not in members or channel in ack:
                    raise ValueError('owner used retired/uninstalled status channel')
                pending = (channel, kind.removeprefix('send_begin_'))
            elif kind.startswith('send_end_'):
                if pending != (channel, kind.removeprefix('send_end_')):
                    raise ValueError('send completion lacks matching synchronous send')
                pending = None
            keys = row['member_ids']
            if keys is not None and (not isinstance(keys, list) or keys != sorted(set(keys))):
                raise ValueError('owner membership identities are not canonical')
            if set(keys or []) != members or row['membership'] is not (channel in members):
                raise ValueError('actual observed map membership differs from transition')
        elif kind in METHOD:
            if row['map_observed'] is not False or row['membership'] is not None or row['member_ids'] is not None:
                raise ValueError('method/observer cannot impersonate owner map observation')
            if kind == 'method_entry':
                if channel in methods or channel not in members:
                    raise ValueError('unexpected method-call population for this owned program')
                methods[channel] = 'entered'
            elif kind == 'method_status_received_after_io':
                if methods.get(channel) != 'entered':
                    raise ValueError('receive hook outside active method')
                # This hook runs AFTER the receive. It can be scheduled after
                # an owner close hook; do not invent IO linearization from it.
            elif kind == 'method_ack_observed':
                if methods.get(channel) != 'entered' or channel not in ack:
                    raise ValueError('actual owner ack not observed before method ack hook')
                methods[channel] = 'ack_observed'
            else:
                if methods.get(channel) != 'ack_observed':
                    raise ValueError('method return-defer lacks an actual owner ack')
                methods[channel] = 'return_defer_observed'
        else:
            raise ValueError('undeclared lifecycle hook event')
    return dict(records=len(rows), installed_ids=sorted(members), pending_send=pending,
                owner_delete_states=deletion, owner_ack_states=ack, method_states=methods,
                finite_prefix_only=True, receive_IO_linearization_proved=False,
                universal_future_nonuse_proved=False, independent_approved=False)
