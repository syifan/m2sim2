/* M2Sim linker script for bare-metal ARM64 benchmarks */

ENTRY(_start)

SECTIONS
{
    . = 0x10000;
    
    .text : {
        *(.text.startup)
        *(.text)
        *(.text.*)
    }
    
    .rodata : {
        *(.rodata)
        *(.rodata.*)
    }
    
    .data : {
        *(.data)
        *(.data.*)
    }
    
    .bss : {
        __bss_start = .;
        *(.bss)
        *(.bss.*)
        *(COMMON)
        __bss_end = .;
    }
    
    . = ALIGN(16);
    _end = .;
}
