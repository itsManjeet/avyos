#!/bin/env python3

import asyncio
import sys

from qemu.qmp import QMPClient, QMPError
from subprocess import Popen, PIPE
from os import path
from time import sleep

THIS_DIR = path.dirname(path.abspath(__file__))

class VirtualMachine:
    SOCKET_PATH = THIS_DIR + "/sock"
    def __init__(self, iso: str) -> None:
        self.iso = iso
        
    async def start(self) -> None:
        await self._boot_machine()
        await self._connect_qmp()
        
    async def stop(self) -> None:
        await self.qmp.disconnect()
        self.process.terminate()

    async def _boot_machine(self) -> None:
        self.process = Popen("qemu-system-x86_64 -qmp unix:{},server=on,wait=off \
                                -cdrom {} \
                                 -nographic".format(self.SOCKET_PATH, self.iso),
                             shell=True,
                             stdout=PIPE,
                             stderr=PIPE)
        sleep(2)
        return_code = self.process.poll()
        if return_code is not None:
            output = self.process.stderr.read().decode('utf-8')
            raise RuntimeError(f"process finished with {return_code}: {output}")
    
    async def _connect_qmp(self) -> None:
        self.qmp = QMPClient('avyos')
        await self.qmp.connect(self.SOCKET_PATH)
        
    async def exec(self, cmd: str, args: dict = None):
        return await self.qmp.execute(cmd, args)
    
    async def get_screen(self):
        await self.qmp.execute('screendump', {
            'filename': '{}/screen.ppm' % THIS_DIR,
        })
        


    async def run(args: list) -> None:
        if len(args) == 1:
            raise RuntimeError("no iso provided")
        
        vm = VirtualMachine(args[1])
        
        print("Starting VM")
        await vm.start()
        
        res = await vm.exec('query-status')
        print(f"STATUS: {res['status']}")
        
        res = await vm.exec('screendump', {
            'filename': '{}/screen.ppm'.format(THIS_DIR),
        })

        print("Stopping VM")
        await vm.stop()
        

asyncio.run(VirtualMachine.run(sys.argv))       
        